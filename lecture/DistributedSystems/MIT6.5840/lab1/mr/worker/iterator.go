package worker

import (
	"container/heap"
	"errors"
	"mr/rpc"
	std_rpc "net/rpc"
)

var (
	ErrEOF = errors.New("reach end of file")
)

type KVReader interface {
	Read(lastKey string, offset int, n int) ([]rpc.KeyValue, error)
	Close() error
}

type WorkerKVReader struct {
	client *std_rpc.Client
	fileId uint64
	id     string
	method string
}

func (r *WorkerKVReader) Read(lastKey string, offset int, n int) ([]rpc.KeyValue, error) {
	args := &rpc.ReadArgs{
		Id:      r.id,
		FileId:  r.fileId,
		LasyKey: lastKey,
		Offset:  offset,
		N:       n,
	}
	reply := &rpc.ReadReply{}
	err := rpc.Call(r.method, r.client, args, reply)
	if err != nil || !reply.S.OK {
		if err == nil {
			err = errors.New(reply.S.Msg)
		}
		return nil, err
	}
	return reply.KeyValueList, nil
}

func (r *WorkerKVReader) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

func NewKVReader(client *std_rpc.Client, fileId uint64, id string, method string) KVReader {
	r := &WorkerKVReader{
		client: client,
		fileId: fileId,
		id:     id,
		method: method,
	}
	return r
}

type Iterator interface {
	Next() error
	Set(seq int, reader KVReader)
	Get() rpc.KeyValue // MUST call Get() after Next() and Next() return nil
}

type IntermediateKVIterator struct {
	reader  KVReader
	buf     []rpc.KeyValue
	lastKey string
	offset  int
	batch   int
	eof     bool
}

func (iter *IntermediateKVIterator) Next() error {
	if len(iter.buf) == 0 && iter.eof {
		return ErrEOF
	}
	if len(iter.buf) == 0 {
		buf, err := iter.reader.Read(iter.lastKey, iter.offset, iter.batch)
		if err != nil {
			return err
		}
		if len(buf) < iter.batch {
			iter.eof = true
		}
		if len(buf) == 0 {
			return ErrEOF
		}
		iter.buf = append(iter.buf, buf...)
		return nil
	} else {
		if iter.buf[0].Key != iter.lastKey {
			iter.lastKey = iter.buf[0].Key
			iter.offset = 0
		}
		iter.offset++
		iter.buf = iter.buf[1:]
		if len(iter.buf) == 0 {
			return iter.Next()
		}
		return nil
	}
}

func (iter *IntermediateKVIterator) Set(seq int, reader KVReader) {
	iter.reader = reader
}

func (iter *IntermediateKVIterator) Get() rpc.KeyValue {
	return iter.buf[0]
}

func NewIntermediateKVIterator(r KVReader) Iterator {
	iter := &IntermediateKVIterator{
		reader:  r,
		buf:     make([]rpc.KeyValue, 0),
		lastKey: "",
		offset:  0,
		batch:   1024, // TODO
		eof:     false,
	}
	return iter
}

type MergeIterator struct {
	iters      []Iterator
	smallHeap  []int
	first      bool
	comparator func(string, string) int
}

func (iter *MergeIterator) Len() int {
	return len(iter.smallHeap)
}

func (iter *MergeIterator) Less(i, j int) bool {
	i = iter.smallHeap[i]
	j = iter.smallHeap[j]
	lhs := iter.iters[i].Get().Key
	rhs := iter.iters[j].Get().Key
	var ret int
	if iter.comparator != nil {
		ret = iter.comparator(lhs, rhs)
	} else {
		if lhs < rhs {
			ret = -1
		} else if lhs > rhs {
			ret = 1
		} else {
			ret = 0
		}
	}
	if ret != 0 {
		if ret == -1 {
			return true
		} else {
			return false
		}
	} else {
		return i < j
	}
}

func (iter *MergeIterator) Swap(i, j int) {
	iter.smallHeap[i], iter.smallHeap[j] = iter.smallHeap[j], iter.smallHeap[i]
}

func (iter *MergeIterator) Push(x interface{}) {
	iter.smallHeap = append(iter.smallHeap, x.(int))
}

func (iter *MergeIterator) Pop() interface{} {
	x := iter.smallHeap[len(iter.smallHeap)-1]
	iter.smallHeap = iter.smallHeap[0 : len(iter.smallHeap)-1]
	return x
}

func (iter *MergeIterator) Next() error {
	if len(iter.smallHeap) == 0 {
		if iter.first {
			iter.first = false
			for i := 0; i < len(iter.iters); i++ {
				if err := iter.iters[i].Next(); err != nil {
					if err != ErrEOF {
						return err
					}
				} else {
					heap.Push(iter, i)
				}
			}
		}
		if len(iter.smallHeap) == 0 {
			return ErrEOF
		}
		return nil
	}

	x := heap.Pop(iter).(int)
	subIter := iter.iters[x]
	if err := subIter.Next(); err != nil {
		if err != ErrEOF {
			return err
		} else {
			if len(iter.smallHeap) == 0 {
				return ErrEOF
			}
		}
	} else {
		heap.Push(iter, x)
	}
	return nil
}

func (iter *MergeIterator) Set(seq int, r KVReader) {
	iter.iters[seq].Set(0 /* dummy sequence number */, r)
}

func (iter *MergeIterator) Get() rpc.KeyValue {
	x := iter.smallHeap[0]
	subIter := iter.iters[x]
	return subIter.Get()
}

func NewMergeIterator(iters []Iterator) Iterator {
	iter := &MergeIterator{
		iters: iters,
		first: true,
	}
	return iter
}
