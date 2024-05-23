package worker

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"mr/fs"
	"mr/rpc"
	"net"
	"net/http"
	std_rpc "net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Worker struct {
	m         sync.Mutex
	stopped   bool
	wg        sync.WaitGroup
	logger    *log.Logger
	loggerF   *os.File
	id        string
	localPath string
	mapF      func(string, string) []rpc.KeyValue
	reduceF   func(string, []string) string
	taskList  []*Task
	localFs   fs.FileSystem
	port      int
}

func (w *Worker) Assign(args *rpc.AssignArgs, reply *rpc.AssignReply) error {
	if args.Id != w.id {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Obsolete version[%s], current version[%s]", args.Id, w.id)
		return nil
	}
	param := TaskParameter{
		TaskCommonParameter: TaskCommonParameter{
			Type:   args.Type,
			TaskId: args.TaskId,
			Logger: w.logger,
			Id:     w.id,
			Wg:     &w.wg,
			FileId: args.FileId,
		},
	}
	if args.Type == rpc.TASK_TYPE_MAP {
		param.MapperTaskParameter = MapperTaskParameter{
			InputPath:     args.InputPath,
			InputFileList: args.InputFileList,
			TotalReduce:   args.TotalReduce,
			LocalPath:     w.localPath,
			MapF:          w.mapF,
		}
	} else {
		param.ReducerTaskParameter = ReducerTaskParameter{
			OutputPath: args.OutputPath,
			ReduceF:    w.reduceF,
			TotalMap:   args.TotalMap,
			Method:     "Worker.Read",
		}
	}
	task := NewTask(param)
	if task == nil {
		reply.S.OK = false
		reply.S.Msg = "Invalid argument"
		return nil
	}
	if err := task.Start(); err != nil {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Fail to start task[%s]", err.Error())
		return nil
	}
	reply.S.OK = true
	w.m.Lock()
	defer w.m.Unlock()
	w.taskList = append(w.taskList, task)
	return nil
}

func (w *Worker) Heartbeat(args *rpc.HeartbeatArgs, reply *rpc.HeartbeatReply) error {
	if args.Id != w.id {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Obsolete version[%s], current version[%s]", args.Id, w.id)
		return nil
	}
	reply.TaskIdList = make([]uint64, 0)
	reply.TaskTypeList = make([]rpc.TaskType, 0)
	reply.TaskStatusList = make([]rpc.TaskStatus, 0)
	w.m.Lock()
	defer w.m.Unlock()
	for _, task := range w.taskList {
		info := task.GetTaskInfo()
		reply.TaskIdList = append(reply.TaskIdList, info.TaskId)
		reply.TaskTypeList = append(reply.TaskTypeList, info.Type)
		reply.TaskStatusList = append(reply.TaskStatusList, info.Status)
	}
	reply.S.OK = true
	return nil
}

func (w *Worker) Read(args *rpc.ReadArgs, reply *rpc.ReadReply) error {
	if args.Id != w.id {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Obsolete version[%s], current version[%s]", args.Id, w.id)
		return nil
	}
	fname := fmt.Sprintf("map-%d", args.FileId)
	fh, err := w.localFs.Open(fname, fs.READ)
	if err != nil {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Fail to open[%s], err[%s]", fname, err.Error())
		return nil
	}
	// TODO use binary search to improve performance
	defer fh.Close()
	lineR := bufio.NewReader(fh)
	var line []byte
	offset := 0
	var item rpc.KeyValue
	reply.KeyValueList = make([]rpc.KeyValue, 0)
	reply.S.OK = true
	for {
		line, _, err = lineR.ReadLine()
		if err != nil {
			if err != io.EOF {
				reply.S.OK = false
				reply.S.Msg = fmt.Sprintf("Fail to readline[%s], err[%s]", fname, err.Error())
			}
			break
		}
		kv := strings.Split(string(line), " ")
		if len(kv) != 2 {
			reply.S.OK = false
			reply.S.Msg = fmt.Sprintf("Invalid format[%s]", string(line))
			break
		}
		if kv[0] == args.LasyKey {
			offset++
		}
		if offset > args.Offset || kv[0] > args.LasyKey {
			item = rpc.KeyValue{
				Key:   kv[0],
				Value: kv[1],
			}
			reply.KeyValueList = append(reply.KeyValueList, item)
			if len(reply.KeyValueList) == args.N {
				break
			}
		}
	}
	return nil
}

func (w *Worker) Notify(args *rpc.NotifyArgs, reply *rpc.NotifyReply) error {
	if args.Id != w.id {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Obsolete version[%s], current version[%s]", args.Id, w.id)
		return nil
	}
	if args.Type != rpc.TASK_TYPE_REDUCE {
		reply.S.OK = false
		reply.S.Msg = "Notify only valid for reducer"
		return nil
	}
	w.logger.Printf("Receive Notify, task[%d], mapper[%d], addr[%s], fileId[%d]",
		args.TaskId, args.MapperTaskId, args.NewAddr, args.NewFileId)
	w.m.Lock()
	defer w.m.Unlock()
	for _, t := range w.taskList {
		info := t.GetTaskInfo()
		if info.Type == rpc.TASK_TYPE_REDUCE && info.TaskId == args.TaskId {
			if err := t.Notify(args); err != nil {
				reply.S.OK = false
				reply.S.Msg = fmt.Sprintf("Fail to notify[%s]", err.Error())
				return nil
			}
		}
	}
	reply.S.OK = true
	return nil
}

func (w *Worker) Destroy(args *rpc.DestroyArgs, reply *rpc.DestroyReply) error {
	if args.Id != w.id {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Obsolete version[%s], current version[%s]", args.Id, w.id)
		return nil
	}
	w.m.Lock()
	defer w.m.Unlock()
	reply.S.OK = true
	if w.stopped {
		// ignore
		return nil
	}
	w.wg.Done()
	return nil
}

func (w *Worker) Init(localPath string,
	mapF func(string, string) []rpc.KeyValue,
	reduceF func(string, []string) string) error {
	w.id = strconv.Itoa(os.Getpid())
	w.stopped = false
	w.mapF = mapF
	w.reduceF = reduceF
	w.taskList = make([]*Task, 0)
	if err := os.Mkdir(filepath.Join(localPath, w.id), 0755); err != nil {
		return err
	}
	// localPath
	//	|
	//  +---- <worker id>
	//			   |
	//			   +---- log		<=== worker log
	//			   |
	//			   +---- data		<=== intermediate data
	localPath = filepath.Join(localPath, w.id)
	if err := os.Mkdir(filepath.Join(localPath, "data"), 0755); err != nil {
		return err
	}
	w.localPath = filepath.Join(localPath, "data")
	if err := os.Mkdir(filepath.Join(localPath, "log"), 0755); err != nil {
		return err
	}
	if localFs, err := fs.NewFileSystem(fmt.Sprintf("local://%s", w.localPath)); err != nil {
		return err
	} else {
		w.localFs = localFs
	}
	if f, err := os.OpenFile(filepath.Join(localPath, "log", "worker.log"), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644); err != nil {
		return err
	} else {
		w.loggerF = f
		w.logger = log.New(f, "", log.Lshortfile|log.LstdFlags)
	}
	// for Destroy
	w.wg.Add(1)
	w.logger.Printf("Succ to init worker[%s]", w.id)
	return nil
}

func (w *Worker) Start() {
	w.port = 8890
	std_rpc.Register(w)
	std_rpc.HandleHTTP()
	for {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", w.port))
		if err != nil {
			w.port++
			continue
		}
		go http.Serve(l, nil)
		time.Sleep(1 * time.Second)
		break
	}
	w.logger.Printf("Succ to listen on %s", w.ListenAddress())
}

func (w *Worker) ListenAddress() string {
	return fmt.Sprintf("127.0.0.1:%d", w.port)
}

func (w *Worker) Done() {
	w.wg.Wait()
	w.logger.Printf("Worker[%s] has stopped, addr[%s]", w.id, w.ListenAddress())
	w.loggerF.Close()
}

func (w *Worker) Id() string {
	return w.id
}
