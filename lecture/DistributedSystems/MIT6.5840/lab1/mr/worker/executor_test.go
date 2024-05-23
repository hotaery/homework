package worker

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mr/rpc"
	"net"
	"net/http"
	std_rpc "net/rpc"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func randomStr(n int) string {
	ans := make([]byte, n)
	alphaTable := "abcdefghijklmnopqrstuvwxyz"
	for i := 0; i < n; i++ {
		ans[i] = alphaTable[rand.Intn(1000)%len(alphaTable)]
	}
	return string(ans)
}

var testKVList []rpc.KeyValue = make([]rpc.KeyValue, 0)

func Map(filename string, content string) []rpc.KeyValue {
	n := rand.Intn(100) + 10
	ans := make([]rpc.KeyValue, 0)
	for i := 0; i < n; i++ {
		ans = append(ans, rpc.KeyValue{
			Key:   randomStr(rand.Intn(16)),
			Value: randomStr(rand.Intn(16)),
		})
	}
	testKVList = append(testKVList, ans...)
	return ans
}

func GenerateFile(dir string, n int) (fList []string, err error) {
	var f *os.File
	fList = make([]string, 0)
	for i := 0; i < n && err == nil; i++ {
		fname := fmt.Sprintf("file-%d", i)
		fList = append(fList, fname)
		fname = filepath.Join(dir, fname)
		f, err = os.Create(fname)
		if f != nil {
			f.Close()
		}
	}
	return
}

func TestExecuteMapper(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		t.Fatalf("MkdirTemp: %s", err.Error())
	}
	defer func() {
		os.RemoveAll(tempDir)
	}()
	localPath, err := os.MkdirTemp("", "*")
	if err != nil {
		t.Fatalf("MkdirLocal: %s", err.Error())
	}
	defer func() {
		os.RemoveAll(localPath)
	}()
	var fList []string
	if fList, err = GenerateFile(tempDir, 10); err != nil {
		t.Fatalf("GenerateFile: %s", err.Error())
	}

	var wg sync.WaitGroup
	param := TaskParameter{
		TaskCommonParameter: TaskCommonParameter{
			Type:   rpc.TASK_TYPE_MAP,
			TaskId: 1,
			Logger: log.Default(),
			Id:     "xxx",
			Wg:     &wg,
			FileId: 1,
		},
		MapperTaskParameter: MapperTaskParameter{
			InputPath:     tempDir,
			InputFileList: fList,
			TotalReduce:   5,
			LocalPath:     localPath,
			MapF:          Map,
		},
	}

	task := NewTask(param)
	if task == nil {
		t.Fatalf("Fail to create task")
	}
	if err = task.Start(); err != nil {
		t.Fatalf("Fail to start, %s", err.Error())
	}
	wg.Wait()

	var children []os.DirEntry
	children, err = os.ReadDir(localPath)
	if err != nil {
		t.Fatalf("ReadDir: %s", err.Error())
	}
	if len(children) != param.TotalReduce {
		t.Fatalf("Mismatch number of intermediate, expect[%d], actual[%d]", param.TotalReduce, len(children))
	}

	readKVList := make([]rpc.KeyValue, 0)
	for i := 0; i < len(children); i++ {
		fname := fmt.Sprintf("map-%d", param.FileId+uint64(i))
		if children[i].Name() != fname {
			t.Fatalf("Mismatch name of intermediate, expect[%s], actual[%s]", fname, children[i].Name())
		}
		var f *os.File
		if f, err = os.Open(filepath.Join(localPath, fname)); err != nil {
			t.Fatalf("Open intermediate[%s], err[%s]", fname, err.Error())
		}
		defer f.Close()
		lineR := bufio.NewReader(f)
		var line []byte
		for {
			line, _, err = lineR.ReadLine()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Readline %s, err[%s]", fname, err.Error())
			}
			lines := string(line)
			kv := strings.Split(lines, " ")
			if len(kv) != 2 {
				t.Fatalf("Wrong format %s, line[%s]", fname, lines)
			}
			readKVList = append(readKVList, rpc.KeyValue{
				Key:   kv[0],
				Value: kv[1],
			})
		}
	}
	if len(readKVList) != len(testKVList) {
		t.Fatalf("Mismatch length, expect[%d], actual[%d]", len(testKVList), len(readKVList))
	}
	readKVList_ := KVList(readKVList)
	testKVList_ := KVList(testKVList)
	sort.Sort(readKVList_)
	sort.Sort(testKVList_)
	for i, expect := range testKVList_ {
		actual := testKVList_[i]
		if expect != actual {
			t.Fatalf("Mismatch KV, expect[%s %s], actual[%s %s]", expect.Key, expect.Value, actual.Key, actual.Value)
		}
	}
}

type MockWorker struct {
	intermediateKV map[int]KVList
	port           int
	delay          bool
}

func (s *MockWorker) Init(nMap int) {
	s.intermediateKV = make(map[int]KVList)
	s.delay = false
	for i := 0; i < nMap; i++ {
		n := rand.Intn(100) + 20
		kvList := KVList(make([]rpc.KeyValue, 0))
		for j := 0; j < n; j++ {
			k := randomStr(rand.Intn(8))
			v := randomStr(rand.Intn(8))
			kvList = append(kvList, rpc.KeyValue{
				Key:   k,
				Value: v,
			})
		}
		sort.Sort(kvList)
		s.intermediateKV[i] = kvList
	}
}

func (s *MockWorker) Read(args *rpc.ReadArgs, reply *rpc.ReadReply) error {
	if s.delay {
		// 500ms
		time.Sleep(500 * time.Millisecond)
	}
	if kvList, ok := s.intermediateKV[int(args.FileId)]; ok {
		index := sort.Search(len(kvList), func(i int) bool {
			return kvList[i].Key >= args.LasyKey
		})
		for i := 0; i < args.Offset && index < len(kvList); i++ {
			index++
		}
		for i := 0; i < args.N && index < len(kvList); i++ {
			reply.KeyValueList = append(reply.KeyValueList, kvList[index])
			index++
		}
		reply.S.OK = true
	} else {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Invalid file id[%d]", args.FileId)
	}
	return nil
}

func (s *MockWorker) Start() {
	s.port = 8890
	for {
		std_rpc.Register(s)
		std_rpc.HandleHTTP()
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
		if err != nil {
			s.port++
			continue
		}
		go http.Serve(l, nil)
		time.Sleep(1 * time.Second)
		break
	}
}

func (s *MockWorker) SetDelay(delay bool) {
	s.delay = delay
}

func (s *MockWorker) ListenAddress() string {
	return fmt.Sprintf("127.0.0.1:%d", s.port)
}

func (s *MockWorker) NMap() int {
	return len(s.intermediateKV)
}

func (s *MockWorker) AllKV() KVList {
	kvList := KVList(make([]rpc.KeyValue, 0))
	for _, v := range s.intermediateKV {
		kvList = append(kvList, v...)
	}
	sort.Sort(kvList)
	return kvList
}

func Reduce(key string, values []string) string {
	return strconv.Itoa(len(values))
}

var srv *MockWorker = nil

func StartWorker() {
	if srv == nil {
		srv = new(MockWorker)
		nMap := rand.Intn(8) + 1
		srv.Init(nMap)
		srv.Start()
	}
}

func TestExecuteReducer(t *testing.T) {
	StartWorker()
	tempDir, _ := os.MkdirTemp("", "*")
	defer func() {
		os.RemoveAll(tempDir)
	}()

	var wg sync.WaitGroup
	param := TaskParameter{
		TaskCommonParameter: TaskCommonParameter{
			Type:   rpc.TASK_TYPE_REDUCE,
			TaskId: 1,
			Logger: log.Default(),
			Id:     "xxx",
			Wg:     &wg,
			FileId: 1,
		},
		ReducerTaskParameter: ReducerTaskParameter{
			OutputPath: tempDir,
			ReduceF:    Reduce,
			TotalMap:   srv.NMap(),
			Method:     "MockWorker.Read",
		},
	}
	task := NewTask(param)
	if task == nil {
		t.Fatalf("NewTask: nil")
	}
	if err := task.Start(); err != nil {
		t.Fatalf("Task::Start: %s", err.Error())
	}

	info := task.GetTaskInfo()
	if info.Status != rpc.TASK_STATUS_IN_PROGRESS {
		t.Fatalf("Abnormal task status")
	}

	for i := 0; i < srv.NMap(); i++ {
		args := &rpc.NotifyArgs{
			OldId:        "",
			NewId:        fmt.Sprintf("zzz-%d", i),
			NewAddr:      srv.ListenAddress(),
			NewFileId:    uint64(i),
			MapperTaskId: uint64(i),
		}
		if err := task.Notify(args); err != nil {
			t.Fatalf("Notify: %d %s", i, err.Error())
		}
	}
	wg.Wait()
	var children []os.DirEntry
	var err error
	if children, err = os.ReadDir(param.OutputPath); err != nil {
		t.Fatalf("Readdir: %s", err.Error())
	}
	if len(children) != 1 {
		t.Fatalf("Mismatch output, expect[%d], actual[%d]", 1, len(children))
	}

	if children[0].Name() != fmt.Sprintf("mr-out-%d", param.FileId) {
		t.Fatalf("Mismatch output file name, expect[%s], actual[%s]", fmt.Sprintf("mr-out-%d", param.FileId), children[0].Name())
	}

	kvList := srv.AllKV()
	fh, _ := os.Open(filepath.Join(param.OutputPath, children[0].Name()))
	lineR := bufio.NewReader(fh)
	idx := 0
	for {
		var line []byte
		if line, _, err = lineR.ReadLine(); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("Readline: %s", err.Error())
		}
		if idx == len(kvList) {
			break
		}
		kv := strings.Split(string(line), " ")
		if len(kv) != 2 {
			t.Fatalf("Invalid format [%s], expect[2], actual[%d]", string(line), len(kv))
		}
		if kv[0] != kvList[idx].Key {
			t.Fatalf("Mismatch key, expect[%s], actual[%s]", kvList[idx].Key, kv[0])
		}
		freq := 0
		for idx < len(kvList) {
			if kvList[idx].Key != kv[0] {
				break
			}
			idx++
			freq++
		}
		actualFreq := 0
		if actualFreq, err = strconv.Atoi(kv[1]); err != nil {
			t.Fatalf("Invalid value[%s], err[%s]", kv[1], err.Error())
		}
		if freq != actualFreq {
			t.Fatalf("Mismatch frequency, expect[%d], actual[%d]", freq, actualFreq)
		}
	}
	if err != io.EOF || idx != len(kvList) {
		t.Fatalf("Expect end of file and iterate all kv, err[%s], idx[%d], len[%d]", err.Error(), idx, len(kvList))
	}
}

func TestExecuteReducerWithRetry(t *testing.T) {
	StartWorker()
	srv.SetDelay(true)
	tempDir, _ := os.MkdirTemp("", "*")
	defer func() {
		os.RemoveAll(tempDir)
	}()

	var wg sync.WaitGroup
	param := TaskParameter{
		TaskCommonParameter: TaskCommonParameter{
			Type:   rpc.TASK_TYPE_REDUCE,
			TaskId: 1,
			Logger: log.Default(),
			Id:     "xxx",
			Wg:     &wg,
			FileId: 1,
		},
		ReducerTaskParameter: ReducerTaskParameter{
			OutputPath: tempDir,
			ReduceF:    Reduce,
			TotalMap:   srv.NMap(),
			Method:     "MockWorker.Read",
		},
	}
	task := NewTask(param)
	if task == nil {
		t.Fatalf("NewTask: nil")
	}
	if err := task.Start(); err != nil {
		t.Fatalf("Task::Start: %s", err.Error())
	}

	info := task.GetTaskInfo()
	if info.Status != rpc.TASK_STATUS_IN_PROGRESS {
		t.Fatalf("Abnormal task status")
	}

	idTable := make([]string, 0)
	idFileIdTable := make(map[string]int)
	for i := 0; i < srv.NMap(); i++ {
		args := &rpc.NotifyArgs{
			OldId:        "",
			NewId:        fmt.Sprintf("zzz-%d", i),
			NewAddr:      srv.ListenAddress(),
			NewFileId:    uint64(i),
			MapperTaskId: uint64(i),
		}
		if err := task.Notify(args); err != nil {
			t.Fatalf("Notify: %d %s", i, err.Error())
		}
		idTable = append(idTable, args.NewId)
		idFileIdTable[args.NewId] = i
	}

	var err error
	go func() {
		idN := srv.NMap()
		for {
			info := task.GetTaskInfo()
			if info.Status != rpc.TASK_STATUS_IN_PROGRESS {
				break
			}
			i := rand.Intn(srv.NMap())
			args := &rpc.NotifyArgs{
				OldId:        idTable[i],
				NewId:        fmt.Sprintf("zzz-%d", idN),
				NewAddr:      srv.ListenAddress(),
				NewFileId:    uint64(idFileIdTable[idTable[i]]),
				MapperTaskId: uint64(idFileIdTable[idTable[i]]),
			}
			if err = task.Notify(args); err != nil {
				break
			}
			idTable[i] = args.NewId
			delete(idFileIdTable, args.OldId)
			idFileIdTable[args.NewId] = int(args.NewFileId)
			idN++
			time.Sleep(1 * time.Second)
		}
	}()

	wg.Wait()
	if err != nil {
		t.Fatalf("Abnormal: %s", err.Error())
	}
	var children []os.DirEntry
	if children, err = os.ReadDir(param.OutputPath); err != nil {
		t.Fatalf("Readdir: %s", err.Error())
	}
	if len(children) != 1 {
		t.Fatalf("Mismatch output, expect[%d], actual[%d]", 1, len(children))
	}
	if children[0].Name() != fmt.Sprintf("mr-out-%d", param.FileId) {
		t.Fatalf("Mismatch output file name, expect[%s], actual[%s]", fmt.Sprintf("mr-out-%d", param.FileId), children[0].Name())
	}

	kvList := srv.AllKV()
	fh, _ := os.Open(filepath.Join(param.OutputPath, children[0].Name()))
	lineR := bufio.NewReader(fh)
	idx := 0
	for {
		var line []byte
		if line, _, err = lineR.ReadLine(); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("Readline: %s", err.Error())
		}
		if idx == len(kvList) {
			break
		}
		kv := strings.Split(string(line), " ")
		if len(kv) != 2 {
			t.Fatalf("Invalid format [%s], expect[2], actual[%d]", string(line), len(kv))
		}
		if kv[0] != kvList[idx].Key {
			t.Fatalf("Mismatch key, expect[%s], actual[%s]", kvList[idx].Key, kv[0])
		}
		freq := 0
		for idx < len(kvList) {
			if kvList[idx].Key != kv[0] {
				break
			}
			idx++
			freq++
		}
		actualFreq := 0
		if actualFreq, err = strconv.Atoi(kv[1]); err != nil {
			t.Fatalf("Invalid value[%s], err[%s]", kv[1], err.Error())
		}
		if freq != actualFreq {
			t.Fatalf("Mismatch frequency, expect[%d], actual[%d]", freq, actualFreq)
		}
	}
	if err != io.EOF || idx != len(kvList) {
		t.Fatalf("Expect end of file and iterate all kv, err[%s], idx[%d], len[%d]", err.Error(), idx, len(kvList))
	}
}
