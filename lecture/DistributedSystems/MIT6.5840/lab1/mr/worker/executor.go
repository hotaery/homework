package worker

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"mr/fs"
	"mr/rpc"
	std_rpc "net/rpc"
	"sort"
	"sync"
)

func DefaultPartitionF(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

type TaskCommonParameter struct {
	Type   rpc.TaskType
	TaskId uint64
	Logger *log.Logger
	Id     string
	Wg     *sync.WaitGroup
	FileId uint64
}

type MapperTaskParameter struct {
	InputPath     string
	InputFileList []string
	TotalReduce   int
	LocalPath     string
	MapF          func(string, string) []rpc.KeyValue
	PartitionF    func(string) int
}

type ReducerTaskParameter struct {
	OutputPath string
	ReduceF    func(string, []string) string
	TotalMap   int
	Method     string
}

type TaskParameter struct {
	TaskCommonParameter
	MapperTaskParameter
	ReducerTaskParameter
}

type TaskInfo struct {
	Type   rpc.TaskType
	TaskId uint64
	Status rpc.TaskStatus
}

type KVList []rpc.KeyValue

func (l KVList) Less(i, j int) bool {
	return l[i].Key < l[j].Key
}

func (l KVList) Len() int {
	return len(l)
}

func (l KVList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type ReaderWithId struct {
	reader KVReader
	id     string
}

type Task struct {
	m        sync.Mutex
	cond     *sync.Cond
	param    TaskParameter
	gfs      fs.FileSystem
	fhs      []fs.FileHandle
	tmpFname string

	// mutable field below
	status       rpc.TaskStatus
	readerList   []ReaderWithId
	iter         Iterator
	finishMapper int
}

func (t *Task) Start() error {
	t.m.Lock()
	defer t.m.Unlock()
	if t.status != rpc.TASK_STATUS_IDLE {
		return errors.New("task has started")
	}
	t.status = rpc.TASK_STATUS_IN_PROGRESS
	t.param.Wg.Add(1)
	if t.param.Type == rpc.TASK_TYPE_MAP {
		go t.executeMapper()
	} else if t.param.Type == rpc.TASK_TYPE_REDUCE {
		go t.executeReducer()
	} else {
		return fmt.Errorf("invalid task type %d", t.param.Type)
	}
	return nil
}

func (t *Task) Notify(args *rpc.NotifyArgs) error {
	client, err := std_rpc.DialHTTP("tcp", args.NewAddr)
	if err != nil {
		t.param.Logger.Printf("Fail to connect worker[%s], err[%s]", args.NewAddr, err.Error())
		return err
	}
	t.m.Lock()
	defer t.m.Unlock()
	if args.MapperTaskId >= uint64(t.param.TotalMap) {
		t.param.Logger.Printf("Fail to notify, invalid mapper task id[%d]", args.MapperTaskId)
		client.Close()
		return fmt.Errorf("invalid mapper task id")
	}
	if args.NewId == t.readerList[args.MapperTaskId].id {
		// ignore
		t.param.Logger.Printf("Receive duplicate notify request, mapper[%d], id[%s]", args.MapperTaskId, args.NewId)
		client.Close()
		return nil
	}
	if t.readerList[args.MapperTaskId].reader == nil {
		t.param.Logger.Printf("Process notify request first time for reducer[%d], id[%s], old[%s], mapper[%d]",
			t.param.TaskId, args.NewId, args.OldId, args.MapperTaskId)
		t.readerList[args.MapperTaskId].reader = NewKVReader(client, args.NewFileId, args.NewId, t.param.Method)
		t.readerList[args.MapperTaskId].id = args.NewId
		t.finishMapper++
		t.cond.Signal()
	} else {
		t.param.Logger.Printf("Process notify request for reducer[%d], id[%s], old[%s], curr[%s], mapper[%d]",
			t.param.TaskId, args.NewId, args.Id, t.readerList[args.MapperTaskId].id, args.MapperTaskId)
		if args.OldId == t.readerList[args.MapperTaskId].id {
			t.readerList[args.MapperTaskId].reader = NewKVReader(client, args.NewFileId, args.NewId, t.param.Method)
			t.readerList[args.MapperTaskId].id = args.NewId
			if t.iter != nil {
				t.iter.Set(int(args.MapperTaskId), t.readerList[args.MapperTaskId].reader)
				t.cond.Signal()
			}
		}
	}
	return nil
}

func (t *Task) clean() {
	for _, r := range t.readerList {
		r.reader.Close()
	}
	for _, f := range t.fhs {
		f.Close()
	}
	t.param.Wg.Done()
}

func (t *Task) writeIntermediate(kvList KVList) (err error) {
	for _, kv := range kvList {
		part := t.param.PartitionF(kv.Key) % t.param.TotalReduce
		_, err = fmt.Fprintf(t.fhs[part], "%s %s\n", kv.Key, kv.Value)
		if err != nil {
			break
		}
	}
	return
}

func (t *Task) executeMapper() {
	var err error
	var fh fs.FileHandle
	var content []byte
	var kvList KVList
	t.param.Logger.Printf("Start to execute mapper[%d], input[%d]", t.param.TaskId, len(t.param.InputFileList))
	for i := 0; i < len(t.param.InputFileList); i++ {
		fname := t.param.InputFileList[i]
		fh, err = t.gfs.Open(fname, fs.READ)
		defer fh.Close()
		if err != nil {
			t.param.Logger.Printf("Fail to open input file[%s], err[%s]", fname, err.Error())
			break
		}
		content, err = io.ReadAll(fh)
		t.param.Logger.Printf("Read input finish[%s], len[%d]", fname, len(content))
		if err != nil {
			t.param.Logger.Printf("Fail to read input file[%s], err[%s]", fname, err.Error())
			break
		}
		kvList = t.param.MapF(fname, string(content))
		t.param.Logger.Printf("Finish to call Mapf, len[%d]", len(kvList))
		sort.Sort(kvList)
		err = t.writeIntermediate(kvList)
		if err != nil {
			t.param.Logger.Printf("Fail to write intermediate file, fname[%s], len[%d], err[%s]", fname, len(kvList), err.Error())
			break
		}
	}
	t.m.Lock()
	defer t.m.Unlock()
	if err != nil {
		t.status = rpc.TASK_STATUS_ERROR
	} else {
		t.status = rpc.TASK_STATUS_COMPLETE
	}
	t.clean()
	t.param.Logger.Printf("Finish to execute mapper[%d], status[%d]", t.param.TaskId, t.status)
}

func (t *Task) writeOutput(key string, value string) (err error) {
	_, err = fmt.Fprintf(t.fhs[0], "%s %s\n", key, value)
	return
}

func (t *Task) executeReducer() {
	t.m.Lock()
	defer t.m.Unlock()
	for t.finishMapper < t.param.TotalMap {
		t.cond.Wait()
	}
	t.param.Logger.Printf("Start to execute reducer, taskid[%d]", t.param.TaskId)
	iters := make([]Iterator, 0)
	for i := 0; i < t.param.TotalMap; i++ {
		iters = append(iters, NewIntermediateKVIterator(t.readerList[i].reader))
	}
	t.iter = NewMergeIterator(iters)
	var currentKey string
	valueList := make([]string, 0)
	var err error
	finish := false
	for !finish {
		var kv rpc.KeyValue
		err = t.iter.Next()
		if err != nil {
			if err == ErrEOF {
				finish = true
				err = nil
			} else {
				t.param.Logger.Printf("Fail to iterate intermediate KV, err[%s]", err.Error())
				// TODO check worker failed
				t.cond.Wait()
				err = nil
				continue
			}
		} else {
			kv = t.iter.Get()
		}
		if len(valueList) > 0 && (finish || kv.Key != currentKey) {
			t.m.Unlock()
			val := t.param.ReduceF(currentKey, valueList)
			err = t.writeOutput(currentKey, val)
			if err != nil {
				t.param.Logger.Printf("Fail to write output, key[%s], err[%s]", currentKey, err.Error())
				t.m.Lock()
				break
			}
			valueList = valueList[:0]
			t.m.Lock()
		}
		if !finish {
			if len(valueList) == 0 {
				currentKey = kv.Key
			}
			valueList = append(valueList, kv.Value)
		}
	}
	if err == nil {
		fname := fmt.Sprintf("mr-out-%d", t.param.FileId)
		err = t.gfs.Rename(t.tmpFname, fname)
	}
	if err != nil {
		t.gfs.Unlink(t.tmpFname)
		t.status = rpc.TASK_STATUS_ERROR
	} else {
		t.status = rpc.TASK_STATUS_COMPLETE
	}
	t.clean()
	t.param.Logger.Printf("Finish to execute reducer[%d], status[%d]", t.param.TaskId, t.status)
}

func (t *Task) GetTaskInfo() TaskInfo {
	t.m.Lock()
	defer t.m.Unlock()
	return TaskInfo{
		Type:   t.param.Type,
		TaskId: t.param.TaskId,
		Status: t.status,
	}
}

func NewTask(param TaskParameter) *Task {
	if param.PartitionF == nil {
		param.PartitionF = DefaultPartitionF
	}
	var err error
	task := new(Task)
	task.cond = sync.NewCond(&task.m)
	task.param = param
	var gfsPath string
	if param.Type == rpc.TASK_TYPE_MAP {
		gfsPath = fmt.Sprintf("local://%s", param.InputPath)
	} else {
		gfsPath = fmt.Sprintf("local://%s", param.OutputPath)
	}

	if task.gfs, err = fs.NewFileSystem(gfsPath); err != nil {
		param.Logger.Printf("Fail to create gfs[%s], err[%s]", gfsPath, err.Error())
		return nil
	}
	task.fhs = make([]fs.FileHandle, 0)
	if param.Type == rpc.TASK_TYPE_MAP {
		localPath := fmt.Sprintf("local://%s", param.LocalPath)
		var localFs fs.FileSystem
		var fh fs.FileHandle
		localFs, err = fs.NewFileSystem(localPath)
		for i := 0; i < param.TotalReduce; i++ {
			fname := fmt.Sprintf("map-%d", param.FileId+uint64(i))
			fh, err = localFs.Open(fname, fs.WRITE)
			if err != nil {
				break
			}
			task.fhs = append(task.fhs, fh)
		}
	} else {
		var fh fs.FileHandle
		// TODO generate unique name
		task.tmpFname = fmt.Sprintf("%s-%d.tmp", param.Id, param.FileId)
		fh, err = task.gfs.Open(task.tmpFname, fs.WRITE)
		if err == nil {
			task.fhs = append(task.fhs, fh)
		}
		task.readerList = make([]ReaderWithId, param.TotalMap)
		task.finishMapper = 0
	}
	if err != nil {
		param.Logger.Printf("Fail to open file, err[%s]", err.Error())
		return nil
	}
	task.status = rpc.TASK_STATUS_IDLE
	task.iter = nil
	return task
}
