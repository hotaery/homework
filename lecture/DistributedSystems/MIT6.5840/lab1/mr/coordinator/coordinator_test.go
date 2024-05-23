package coordinator

import (
	"fmt"
	"mr/rpc"
	std_rpc "net/rpc"
	"os"
	"sync"
	"testing"
	"time"
)

type reducerFinishedStatus struct {
	taskId      uint64
	mapperFlags []int
}

func (r *reducerFinishedStatus) finish(id int) {
	r.mapperFlags[id] = 0
}

func (r *reducerFinishedStatus) finished() bool {
	val := 0
	for _, v := range r.mapperFlags {
		val += v
	}
	return val == 0
}

type MockWorker struct {
	m                   sync.Mutex
	mapperTaskIdList    map[string][]uint64
	reducerTaskFinished map[string][]*reducerFinishedStatus
	finishWorkerId      string
	failedWorkerId      string
	nMap                int
	persisstence        bool
}

func (w *MockWorker) Assign(args *rpc.AssignArgs, reply *rpc.AssignReply) error {
	w.m.Lock()
	defer w.m.Unlock()
	if _, ok := w.mapperTaskIdList[args.Id]; !ok {
		w.mapperTaskIdList[args.Id] = make([]uint64, 0)
	}
	if _, ok := w.reducerTaskFinished[args.Id]; !ok {
		w.reducerTaskFinished[args.Id] = make([]*reducerFinishedStatus, 0)
	}
	if args.Type == rpc.TASK_TYPE_MAP {
		w.mapperTaskIdList[args.Id] = append(w.mapperTaskIdList[args.Id], args.TaskId)
	}
	if args.Type == rpc.TASK_TYPE_REDUCE {
		r := &reducerFinishedStatus{
			taskId:      args.TaskId,
			mapperFlags: make([]int, 0),
		}
		for i := 0; i < w.nMap; i++ {
			r.mapperFlags = append(r.mapperFlags, 1)
		}
		w.reducerTaskFinished[args.Id] = append(w.reducerTaskFinished[args.Id], r)
	}
	reply.S.OK = true
	return nil
}

func (w *MockWorker) Heartbeat(args *rpc.HeartbeatArgs, reply *rpc.HeartbeatReply) error {
	reply.S.OK = true
	w.m.Lock()
	defer w.m.Unlock()
	if args.Id == w.failedWorkerId {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("mock %s crashed", args.Id)
		w.failedWorkerId = ""
		return nil
	}
	reply.TaskIdList = make([]uint64, 0)
	reply.TaskTypeList = make([]rpc.TaskType, 0)
	reply.TaskStatusList = make([]rpc.TaskStatus, 0)
	for id, tasks := range w.mapperTaskIdList {
		if id != args.Id {
			continue
		}
		for _, i := range tasks {
			reply.TaskIdList = append(reply.TaskIdList, i)
			reply.TaskTypeList = append(reply.TaskTypeList, rpc.TASK_TYPE_MAP)
			if id == w.finishWorkerId {
				reply.TaskStatusList = append(reply.TaskStatusList, rpc.TASK_STATUS_COMPLETE)
			} else {
				reply.TaskStatusList = append(reply.TaskStatusList, rpc.TASK_STATUS_IN_PROGRESS)
			}
		}
	}
	for id, tasks := range w.reducerTaskFinished {
		if id != args.Id {
			continue
		}
		for _, i := range tasks {
			reply.TaskIdList = append(reply.TaskIdList, i.taskId)
			reply.TaskTypeList = append(reply.TaskTypeList, rpc.TASK_TYPE_REDUCE)
			if id == w.finishWorkerId && i.finished() {
				reply.TaskStatusList = append(reply.TaskStatusList, rpc.TASK_STATUS_COMPLETE)
			} else {
				reply.TaskStatusList = append(reply.TaskStatusList, rpc.TASK_STATUS_IN_PROGRESS)
			}
		}
	}
	// clear finish worker
	if args.Id == w.finishWorkerId && !w.persisstence {
		w.finishWorkerId = ""
	}
	fmt.Printf("Send heartbeat response to %s, [%v]\n", args.Id, reply)
	return nil
}

func (w *MockWorker) Destroy(args *rpc.DestroyArgs, reply *rpc.DestroyReply) error {
	reply.S.OK = true
	return nil
}

func (w *MockWorker) Notify(args *rpc.NotifyArgs, reply *rpc.NotifyReply) error {
	w.m.Lock()
	defer w.m.Unlock()
	reply.S.OK = true
	if tasks, ok := w.reducerTaskFinished[args.Id]; ok {
		for _, t := range tasks {
			if t.taskId == args.TaskId {
				t.finish(int(args.MapperTaskId))
			}
		}
	} else {
		reply.S.OK = false
		reply.S.Msg = "Internal error, task has not finished"
	}
	return nil
}

func (w *MockWorker) getNumberOfMapper(worker string) int {
	w.m.Lock()
	defer w.m.Unlock()
	if w, ok := w.mapperTaskIdList[worker]; ok {
		return len(w)
	}
	return 0
}

func (w *MockWorker) getNumberOfReducer(worker string) int {
	w.m.Lock()
	defer w.m.Unlock()
	if w, ok := w.reducerTaskFinished[worker]; ok {
		return len(w)
	}
	return 0
}

func (w *MockWorker) setFinishWorker(worker string) {
	w.m.Lock()
	defer w.m.Unlock()
	w.finishWorkerId = worker
}

func (w *MockWorker) setFailedWorker(worker string) {
	w.m.Lock()
	defer w.m.Unlock()
	w.failedWorkerId = worker
}

func TestCoordinator(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "*")
	defer func() {
		fmt.Printf("remove %s\n", tempDir)
		os.RemoveAll(tempDir)
	}()
	c := &Coordinator{}
	options := CoordinatorOptions{
		LocalPath:         tempDir,
		InputPath:         "MockInputPath",
		InputFileList:     make([]string, 0),
		ServiceName:       "MockWorker",
		MaxNumberOfWorker: 2,
		NReduce:           2,
		Networking:        "unix",
	}
	// regitser worker
	worker := &MockWorker{
		mapperTaskIdList:    make(map[string][]uint64),
		reducerTaskFinished: make(map[string][]*reducerFinishedStatus),
		nMap:                4,
		persisstence:        false,
	}
	std_rpc.Register(worker)
	// 4 mapper and 2 reducer
	for i := 0; i < 4; i++ {
		options.InputFileList = append(options.InputFileList, fmt.Sprintf("file-%d", i))
	}
	if err := c.Init(options); err != nil {
		t.Fatalf("Fail to init coordinator, err[%s]", err.Error())
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Fail to start coordinator, err[%s]", err.Error())
	}
	client, err := std_rpc.DialHTTP("unix", rpc.GetMasterAddress())
	if err != nil {
		t.Fatalf("Fail to connect master, err[%s]", err.Error())
	}
	defer client.Close()
	// register worker-1
	{
		args := &rpc.RegisterArgs{
			Id:   "worker-1",
			Addr: rpc.GetMasterAddress(),
		}
		reply := &rpc.RegisterReply{}
		err := rpc.Call("Coordinator.Register", client, args, reply)
		if err != nil || !reply.S.OK {
			t.Fatalf("Fail to register worker-1, err[%v], msg[%s]", err, reply.S.Msg)
		}
	}
	time.Sleep(1 * time.Second)
	if n := worker.getNumberOfMapper("worker-1"); n != 2 {
		t.Fatalf("Worker[worker-1] should have two mapper task, but[%d]", n)
	}
	// register worker-2
	{
		args := &rpc.RegisterArgs{
			Id:   "worker-2",
			Addr: rpc.GetMasterAddress(),
		}
		reply := &rpc.RegisterReply{}
		err := rpc.Call("Coordinator.Register", client, args, reply)
		if err != nil || !reply.S.OK {
			t.Fatalf("Fail to register worker-1, err[%v], msg[%s]", err, reply.S.Msg)
		}
	}
	time.Sleep(1 * time.Second)
	if n := worker.getNumberOfMapper("worker-2"); n != 2 {
		t.Fatalf("Worker[worker-2] should have two mapper task, but[%d]", n)
	}

	// finish worker-1
	worker.setFinishWorker("worker-1")
	fmt.Printf("set finish worker-1\n")
	// assign reducer task to worker-1, sleep 3s to ensure that
	// coordinator can detect completed tasks.
	time.Sleep(3 * time.Second)
	if n := worker.getNumberOfReducer("worker-1"); n != 2 {
		t.Fatalf("Worker[worker-1] should have two reducer task, but[%d]", n)
	}

	// two mappers has completed and two reducers is running at worker-1
	// mark worker-2 failed so two mappers will remove to worker-1
	worker.setFailedWorker("worker-2")
	time.Sleep(3 * time.Second)
	if n := worker.getNumberOfMapper("worker-1"); n != 4 {
		t.Fatalf("Worker[worker-1] should have four mapper tasks, but[%d]", n)
	}

	worker.persisstence = true
	worker.setFinishWorker("worker-1")
	fmt.Printf("set finish worker-1")
	c.Done()
}
