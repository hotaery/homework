package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"mr/rpc"
	"sort"
	"strconv"
	"testing"
)

type DummyKVReader struct {
	kvList    []rpc.KeyValue
	readIndex int
}

func (r *DummyKVReader) Read(lastKey string, offset int, n int) ([]rpc.KeyValue, error) {
	lastKey_ := 0
	var err error
	if len(lastKey) > 0 {
		if lastKey_, err = strconv.Atoi(lastKey); err != nil {
			return nil, err
		}
	}
	var key int
	index := sort.Search(len(r.kvList), func(i int) bool {
		if key, err = strconv.Atoi(r.kvList[i].Key); err != nil {
			return false
		}
		return key >= lastKey_
	})
	if err != nil {
		return nil, err
	}
	if index < len(r.kvList) && r.kvList[index].Key == lastKey {
		for offset > 0 {
			if r.kvList[index].Key != lastKey {
				return nil, errors.New("Invalid offset")
			}
			index++
			offset--
		}
	}
	if index != r.readIndex {
		msg := fmt.Sprintf("Invalid index: expect %d, actual %d\n", r.readIndex, index)
		return nil, errors.New(msg)
	}
	ans := make([]rpc.KeyValue, 0)
	end := min(len(r.kvList), index+n)
	ans = append(ans, r.kvList[index:end]...)
	r.readIndex += len(ans)
	return ans, nil
}

func (r *DummyKVReader) Close() error {
	return nil
}

func (r *DummyKVReader) Init(n int) {
	start := rand.Intn(256)
	for i := 0; i < n; i++ {
		value := rand.Intn(100 * 100)
		kv := rpc.KeyValue{
			Key:   strconv.Itoa(start),
			Value: strconv.Itoa(value),
		}
		r.kvList = append(r.kvList, kv)
		step := rand.Intn(8)
		if start+step < start {
			fmt.Printf("Integer overflow!!!")
		}
		start += step
	}
	r.readIndex = 0
}

func (r *DummyKVReader) AdjustReadIndex(lastKey string, offset int) error {
	lastKey_ := 0
	var err error
	if len(lastKey) > 0 {
		if lastKey_, err = strconv.Atoi(lastKey); err != nil {
			return err
		}
	}
	var key int
	index := sort.Search(len(r.kvList), func(i int) bool {
		if key, err = strconv.Atoi(r.kvList[i].Key); err != nil {
			return false
		}
		return key >= lastKey_
	})
	if err != nil {
		return err
	}
	r.readIndex = index
	for i := 0; i < offset && r.readIndex < len(r.kvList) && r.kvList[r.readIndex].Key == lastKey; i++ {
		r.readIndex++
	}
	return nil
}

func TestIntermediateKVIterator(t *testing.T) {
	r := &DummyKVReader{}
	n := rand.Intn(1000) + 1000
	r.Init(n)
	iter := &IntermediateKVIterator{
		reader:  r,
		buf:     make([]rpc.KeyValue, 0),
		lastKey: "",
		offset:  0,
		batch:   32,
		eof:     false,
	}
	kvList := make([]rpc.KeyValue, 0)
	var err error
	for err = iter.Next(); err == nil; err = iter.Next() {
		kvList = append(kvList, iter.Get())
	}
	if err != ErrEOF {
		t.Fatalf("Abonomal case: %s", err.Error())
	}
	if len(kvList) != n {
		t.Fatalf("Mismatch length, %d", len(kvList))
	}
	for i := 0; i < n; i++ {
		if r.kvList[i] != kvList[i] {
			t.Fatalf("%d not equal, expect %v, actual %v", i, r.kvList[i], kvList[i])
		}
	}
}

func TestIntermediateKVIteratorWithSet(t *testing.T) {
	r := &DummyKVReader{}
	n := rand.Intn(1000) + 1000
	r.Init(n)
	iter := &IntermediateKVIterator{
		reader:  r,
		buf:     make([]rpc.KeyValue, 0),
		lastKey: "",
		offset:  0,
		batch:   32,
		eof:     false,
	}

	breakR := &DummyKVReader{
		kvList: r.kvList,
	}

	breakNumber := rand.Intn(1000) + 500
	kvList := make([]rpc.KeyValue, 0)
	var err error
	for err = iter.Next(); err == nil; err = iter.Next() {
		kvList = append(kvList, iter.Get())
		if len(kvList) == breakNumber {
			iter.Set(0 /* dummy sequence number */, breakR)
			breakR.readIndex = r.readIndex
		}
	}
	if err != ErrEOF {
		t.Fatalf("Abonomal case: %s", err.Error())
	}
	if len(kvList) != n {
		t.Fatalf("Mismatch length, %d", len(kvList))
	}
	for i := 0; i < n; i++ {
		if r.kvList[i] != kvList[i] {
			t.Fatalf("%d not equal, expect %v, actual %v", i, r.kvList[i], kvList[i])
		}
	}
}

func KeyCompare(lhs, rhs string) int {
	lhs_, _ := strconv.Atoi(lhs)
	rhs_, _ := strconv.Atoi(rhs)
	if lhs_ < rhs_ {
		return -1
	} else if lhs_ == rhs_ {
		return 0
	} else {
		return 1
	}
}

type TestKVList []rpc.KeyValue

func (l TestKVList) Less(i, j int) bool {
	ret := KeyCompare(l[i].Key, l[j].Key)
	if ret >= 0 {
		return false
	} else {
		return true
	}
}

func (l TestKVList) Len() int {
	return len(l)
}

func (l TestKVList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func TestMergeIterator(t *testing.T) {
	iters := make([]Iterator, 0)
	itern := rand.Intn(16) + 3
	total := 0
	var allKVList TestKVList = make([]rpc.KeyValue, 0)
	for i := 0; i < itern; i++ {
		r := &DummyKVReader{}
		n := rand.Intn(1000) + 1000
		r.Init(n)
		iter := &IntermediateKVIterator{
			reader:  r,
			buf:     make([]rpc.KeyValue, 0),
			lastKey: "",
			offset:  0,
			batch:   32,
			eof:     false,
		}
		iters = append(iters, iter)
		total += n
		allKVList = append(allKVList, r.kvList...)
	}
	iter := &MergeIterator{
		iters:      iters,
		first:      true,
		comparator: KeyCompare,
	}
	kvList := make([]rpc.KeyValue, 0)
	var err error
	for err = iter.Next(); err == nil; err = iter.Next() {
		kvList = append(kvList, iter.Get())
	}
	if err != ErrEOF {
		t.Fatalf("Abonomal case: %s", err.Error())
	}
	if len(kvList) != total {
		t.Fatalf("Mismatch length, %d", len(kvList))
	}
	sort.Sort(allKVList)
	for i := 0; i < total; i++ {
		if kvList[i].Key != allKVList[i].Key {
			t.Fatalf("%d not equal, expect %v, actual %v", i, allKVList[i], kvList[i])
		}
	}
}
