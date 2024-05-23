package coordinator

import (
	"fmt"
	"log"
	"mr/rpc"
	"net"
	"net/http"
	std_rpc "net/rpc"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type workerStatus int

const (
	WORKER_HEALTH workerStatus = iota
	WORKER_CRASH
)

type workerInfo struct {
	status            workerStatus
	id                string
	addr              string
	mapperTaskIdList  []uint64
	reducerTaskIdList []uint64
	client            *std_rpc.Client
}

type taskInfo struct {
	status       rpc.TaskStatus
	taskType     rpc.TaskType
	fileId       uint64
	taskId       uint64
	workerIdx    int
	oldWorkerIdx int
}

type Coordinator struct {
	m                      sync.Mutex
	scheduleCond           *sync.Cond
	workerInfoList         []workerInfo
	mapperTaskInfoList     []taskInfo
	reducerTaskInfoList    []taskInfo
	unfinishReducerTaskNum int
	wait                   sync.WaitGroup

	// Immutable field
	inputFileList         []string
	inputPath             string
	outputPath            string
	serviceName           string
	logger                *log.Logger
	maxTaskNumberOfWorker int
	loggerF               *os.File
	nReduce               int
	networking            string
}

// Mark worker crash and we should remove this worker
// but may have invalid memroy access in corner case:
//
//	step1. master chooses a worker to assign task
//	step2. this worker is crashed
//	step3. heartbeat goroutine remove this worker
//	step4. master assign task by worker.client  <=== invalid memory access because worker has GC
//
// We have two ways to solve this problem
//   - Never remove. implement easily but waste memory and time consumed when iterate worker list
//   - Using double buffering map: background and foreground. Swap background and foreground after updates has commited
//
// For simplicity, use first solution :)
func (c *Coordinator) markCrashWorker(idx int) {
	c.m.Lock()
	defer c.m.Unlock()
	c.workerInfoList[idx].status = WORKER_CRASH
	for _, taskId := range c.workerInfoList[idx].mapperTaskIdList {
		c.mapperTaskInfoList[taskId].status = rpc.TASK_STATUS_IDLE
		c.mapperTaskInfoList[taskId].oldWorkerIdx = idx
		c.mapperTaskInfoList[taskId].workerIdx = -1
	}
	for _, taskId := range c.workerInfoList[idx].reducerTaskIdList {
		if c.reducerTaskInfoList[taskId].status != rpc.TASK_STATUS_COMPLETE {
			c.reducerTaskInfoList[taskId].status = rpc.TASK_STATUS_IDLE
			c.reducerTaskInfoList[taskId].oldWorkerIdx = idx
			c.reducerTaskInfoList[taskId].workerIdx = -1
		}
	}
}

func (c *Coordinator) finishMapper(workerIdx int, taskId uint64) bool {
	c.m.Lock()
	defer c.m.Unlock()
	if c.mapperTaskInfoList[taskId].status != rpc.TASK_STATUS_IN_PROGRESS {
		// ignore
		// c.logger.Printf("Unexpect error: idle mapper task but finished, taskId[%d]", taskId)
		return false
	}
	if c.mapperTaskInfoList[taskId].workerIdx != workerIdx {
		// ignore
		c.logger.Printf("Unexpect error: mapper has mismatch workerIdx[%d:%d], taskId[%d]", workerIdx,
			c.mapperTaskInfoList[taskId].workerIdx, taskId)
		return false
	}
	c.logger.Printf("Mapper[%d] has completed at worker[%d], addr[%s], id[%s]",
		taskId, workerIdx, c.workerInfoList[workerIdx].addr, c.workerInfoList[workerIdx].id)
	c.mapperTaskInfoList[taskId].status = rpc.TASK_STATUS_COMPLETE
	return true
}

func (c *Coordinator) finishReducer(workerIdx int, taskId uint64) bool {
	c.m.Lock()
	defer c.m.Unlock()
	if c.reducerTaskInfoList[taskId].status != rpc.TASK_STATUS_IN_PROGRESS {
		// ignore
		if c.reducerTaskInfoList[taskId].status != rpc.TASK_STATUS_COMPLETE {
			c.logger.Printf("Unexpect error: idle reducer task but finished, taskId[%d]", taskId)
		}
		return false
	}
	if c.reducerTaskInfoList[taskId].workerIdx != workerIdx {
		// ignore
		c.logger.Printf("Unexpect error: reducer has mismatch workerIdx, taskId[%d]", taskId)
		return false
	}
	c.logger.Printf("Reducer[%d] has completed at worker[%d], addr[%s], id[%s]",
		taskId, workerIdx, c.workerInfoList[workerIdx].addr, c.workerInfoList[workerIdx].id)
	c.unfinishReducerTaskNum--
	c.reducerTaskInfoList[taskId].status = rpc.TASK_STATUS_COMPLETE
	return true
}

func (c *Coordinator) heartbeat(idx int, wg *sync.WaitGroup, active *int32) {
	defer wg.Done()
	args := &rpc.HeartbeatArgs{
		Id: c.workerInfoList[idx].id,
	}
	reply := &rpc.HeartbeatReply{}
	methodName := fmt.Sprintf("%s.Heartbeat", c.serviceName)
	err := rpc.Call(methodName, c.workerInfoList[idx].client, args, reply)
	// TODO add active timeout parameter to determine whether worker has crashed
	if err != nil || !reply.S.OK {
		c.logger.Printf("worker[%s] crashed, addr[%s], err[%v], msg[%s]",
			args.Id, c.workerInfoList[idx].addr, err, reply.S.Msg)
		c.markCrashWorker(idx)
		atomic.AddInt32(active, 1)
	} else {
		for i := 0; i < len(reply.TaskIdList); i++ {
			if reply.TaskStatusList[i] == rpc.TASK_STATUS_COMPLETE {
				var et bool
				if reply.TaskTypeList[i] == rpc.TASK_TYPE_MAP {
					et = c.finishMapper(idx, reply.TaskIdList[i])
				} else {
					et = c.finishReducer(idx, reply.TaskIdList[i])
				}
				if et {
					atomic.AddInt32(active, 1)
				}
			}
		}
	}
}

func (c *Coordinator) startHeartbeat() {
	for {
		var wg sync.WaitGroup
		var active int32 = 0
		{
			// Must lock else conflict with Register()
			c.m.Lock()
			if c.unfinishReducerTaskNum == 0 {
				c.m.Unlock()
				break
			}

			for i, w := range c.workerInfoList {
				if w.status == WORKER_HEALTH {
					wg.Add(1)
					go c.heartbeat(i, &wg, &active)
				}
			}
			c.m.Unlock()
		}
		// Every worker will receive only one heartbeat every cycle
		wg.Wait()
		if active > 0 {
			// Detect some meaningful event, wakeup schedule thread to assign task
			c.scheduleCond.Signal()
		}
		time.Sleep(1 * time.Second)
	}
	c.logger.Printf("Heartbeat goroutine has stopped")
	c.wait.Done()
}

// return -1 means no way to choose worker
func (c *Coordinator) unsafePickWorker(isMapper bool) int {
	minTaskNumber := 0
	workerIdx := -1
	for i, worker := range c.workerInfoList {
		if worker.status == WORKER_HEALTH {
			taskNum := 0
			for _, j := range worker.mapperTaskIdList {
				if c.mapperTaskInfoList[j].status == rpc.TASK_STATUS_IN_PROGRESS {
					taskNum++
				}
			}
			for _, j := range worker.reducerTaskIdList {
				if c.reducerTaskInfoList[j].status == rpc.TASK_STATUS_IN_PROGRESS {
					taskNum++
				}
			}
			if workerIdx == -1 || taskNum < minTaskNumber {
				workerIdx = i
				minTaskNumber = taskNum
			}
		}
	}
	if isMapper {
		// Maybe reducer is waitting for intermediate KV so don't block mapper
		for _, t := range c.reducerTaskInfoList {
			if t.status != rpc.TASK_STATUS_IDLE {
				return workerIdx
			}
		}
	}
	if minTaskNumber >= c.maxTaskNumberOfWorker {
		return -1
	}
	return workerIdx
}

func (c *Coordinator) assignMapper(taskId int, workerIdx int, worker *workerInfo) {
	args := &rpc.AssignArgs{
		Id:            worker.id,
		Type:          rpc.TASK_TYPE_MAP,
		InputFileList: []string{c.inputFileList[taskId]},
		TaskId:        uint64(taskId),
		InputPath:     c.inputPath,
		OutputPath:    "",
		FileId:        c.mapperTaskInfoList[taskId].fileId,
		TotalReduce:   c.nReduce,
	}
	reply := &rpc.AssignReply{}
	method := fmt.Sprintf("%s.Assign", c.serviceName)
	err := rpc.Call(method, worker.client, args, reply)
	if err != nil || !reply.S.OK {
		c.logger.Printf("Fail to assign mapper task to worker[%s], addr[%s], err[%v], msg[%s]",
			worker.id, worker.addr, err, reply.S.Msg)
	} else {
		c.logger.Printf("Succ to assign mapper[%d] task to worker[%s], addr[%s], workderIdx[%d]",
			taskId, args.Id, worker.addr, workerIdx)
	}
}

func (c *Coordinator) assignReducer(taskId int, workerIdx int, worker *workerInfo) {
	args := &rpc.AssignArgs{
		Id:          worker.id,
		Type:        rpc.TASK_TYPE_REDUCE,
		TaskId:      uint64(taskId),
		InputPath:   c.inputPath,
		OutputPath:  c.outputPath,
		FileId:      c.mapperTaskInfoList[taskId].fileId,
		TotalReduce: c.nReduce,
		TotalMap:    len(c.mapperTaskInfoList),
	}
	reply := &rpc.AssignReply{}
	method := fmt.Sprintf("%s.Assign", c.serviceName)
	err := rpc.Call(method, worker.client, args, reply)
	if err != nil || !reply.S.OK {
		// fail to assign task to worker, this abnormal case will be detected by heartbeat goroutine
		// and then will transfrom status of task to idle and reschedule.
		c.logger.Printf("Fail to assign reducer task to worker[%s], addr[%s], err[%v], msg[%s]",
			worker.id, worker.addr, err, reply.S.Msg)
	} else {
		c.logger.Printf("Succ to assign reducer[%d] task to worker[%s], addr[%s], workerIdx[%d]",
			taskId, args.Id, worker.addr, workerIdx)
	}
}

func (c *Coordinator) scheduleTask() {
	c.m.Lock()
	defer c.m.Unlock()
	for {
		if c.unfinishReducerTaskNum == 0 {
			break
		}
		for i, task := range c.mapperTaskInfoList {
			if task.status == rpc.TASK_STATUS_IDLE {
				workerIdx := c.unsafePickWorker(true)
				if workerIdx != -1 {
					c.mapperTaskInfoList[i].status = rpc.TASK_STATUS_IN_PROGRESS
					c.workerInfoList[workerIdx].mapperTaskIdList =
						append(c.workerInfoList[workerIdx].mapperTaskIdList, uint64(i))
					c.mapperTaskInfoList[i].workerIdx = workerIdx
					go c.assignMapper(i, workerIdx, &c.workerInfoList[workerIdx])
				}
			}
		}
		for i, task := range c.reducerTaskInfoList {
			if task.status == rpc.TASK_STATUS_IDLE {
				workerIdx := c.unsafePickWorker(false)
				if workerIdx != -1 {
					c.reducerTaskInfoList[i].status = rpc.TASK_STATUS_IN_PROGRESS
					c.workerInfoList[workerIdx].reducerTaskIdList =
						append(c.workerInfoList[workerIdx].reducerTaskIdList, uint64(i))
					c.reducerTaskInfoList[i].workerIdx = workerIdx
					go c.assignReducer(i, workerIdx, &c.workerInfoList[workerIdx])
				}
			}
		}
		// One of three cases below has happened would wakeup schedule goroutine:
		// 	1. Register worker
		// 	2. Worker has crashed so some task transform to idle
		// 	3. Some task has finished
		c.scheduleCond.Wait()
	}
	c.logger.Printf("Scheduler goroutine has stopped")
	c.wait.Done()
}

func (c *Coordinator) startNotify() {
	method := fmt.Sprintf("%s.Notify", c.serviceName)
	for {
		c.m.Lock()
		if c.unfinishReducerTaskNum == 0 {
			c.m.Unlock()
			break
		}
		var wg sync.WaitGroup
		for _, t := range c.mapperTaskInfoList {
			if t.status == rpc.TASK_STATUS_COMPLETE {
				old := ""
				if t.oldWorkerIdx != -1 {
					old = c.workerInfoList[t.oldWorkerIdx].id
				}
				new := c.workerInfoList[t.workerIdx].id
				addr := c.workerInfoList[t.workerIdx].addr
				for i, reducer := range c.reducerTaskInfoList {
					if reducer.status == rpc.TASK_STATUS_IN_PROGRESS {
						args := &rpc.NotifyArgs{
							Id:           c.workerInfoList[reducer.workerIdx].id,
							Type:         rpc.TASK_TYPE_REDUCE,
							TaskId:       reducer.taskId,
							OldId:        old,
							NewId:        new,
							NewAddr:      addr,
							NewFileId:    t.fileId + uint64(i),
							MapperTaskId: t.taskId,
						}
						reply := &rpc.NotifyReply{}
						wg.Add(1)
						go func(worker int) {
							err := rpc.Call(method, c.workerInfoList[worker].client, args, reply)
							if err != nil || !reply.S.OK {
								c.logger.Printf("Fail to notify, worker[%s], reducer[%d], err[%v], msg[%s]",
									args.Id, args.TaskId, err, reply.S.Msg)
							}
							wg.Done()
						}(reducer.workerIdx)
					}
				}
			}
		}
		c.m.Unlock()
		wg.Wait()
		time.Sleep(1 * time.Second)
	}
	c.logger.Printf("Notify goroutine has finished")
	c.wait.Done()
}

func (c *Coordinator) Register(args *rpc.RegisterArgs, reply *rpc.RegisterReply) error {
	c.logger.Printf("Receive regiser, id[%s], add[%s]", args.Id, args.Addr)
	c.m.Lock()
	defer c.m.Unlock()
	if c.unfinishReducerTaskNum == 0 {
		reply.S.OK = false
		reply.S.Msg = "Coordinator has finished"
		return nil
	}
	reply.S.OK = true
	for _, w := range c.workerInfoList {
		if w.id == args.Id {
			// ignore retry register request
			return nil
		}
	}
	client, err := std_rpc.DialHTTP(c.networking, args.Addr)
	if err != nil {
		reply.S.OK = false
		reply.S.Msg = fmt.Sprintf("Fail to dial[%s], err[%s], networking[%s]", args.Addr, err.Error(), c.networking)
		return nil
	}
	worker := workerInfo{
		status:            WORKER_HEALTH,
		id:                args.Id,
		addr:              args.Addr,
		mapperTaskIdList:  make([]uint64, 0),
		reducerTaskIdList: make([]uint64, 0),
		client:            client,
	}
	c.workerInfoList = append(c.workerInfoList, worker)
	c.scheduleCond.Signal()
	return nil
}

type CoordinatorOptions struct {
	LocalPath         string
	InputPath         string
	OutputPath        string
	InputFileList     []string
	ServiceName       string
	MaxNumberOfWorker int
	NReduce           int
	Networking        string
}

func (c *Coordinator) Init(options CoordinatorOptions) error {
	// init field of Coordinator
	c.inputFileList = options.InputFileList
	c.inputPath = options.InputPath
	c.outputPath = options.OutputPath
	c.maxTaskNumberOfWorker = options.MaxNumberOfWorker
	c.serviceName = options.ServiceName
	c.scheduleCond = sync.NewCond(&c.m)
	c.workerInfoList = make([]workerInfo, 0)
	c.mapperTaskInfoList = make([]taskInfo, 0)
	c.reducerTaskInfoList = make([]taskInfo, 0)
	c.nReduce = options.NReduce
	c.networking = options.Networking

	// init mapper task and reducer task
	for i := range c.inputFileList {
		t := taskInfo{
			status:       rpc.TASK_STATUS_IDLE,
			taskType:     rpc.TASK_TYPE_MAP,
			fileId:       uint64(i * options.NReduce),
			taskId:       uint64(i),
			workerIdx:    -1,
			oldWorkerIdx: -1,
		}
		c.mapperTaskInfoList = append(c.mapperTaskInfoList, t)
	}

	for i := 0; i < options.NReduce; i++ {
		t := taskInfo{
			status:       rpc.TASK_STATUS_IDLE,
			taskType:     rpc.TASK_TYPE_REDUCE,
			fileId:       uint64(i),
			taskId:       uint64(i),
			workerIdx:    -1,
			oldWorkerIdx: -1,
		}
		c.reducerTaskInfoList = append(c.reducerTaskInfoList, t)
	}
	c.unfinishReducerTaskNum = len(c.reducerTaskInfoList)

	// init logger
	// <localPath>
	//	 |
	//	 + coordinator		<=== coordinator directory
	//	 |
	//	 + <worker id>		<=== worker directory
	var err error
	localPath := filepath.Join(options.LocalPath, "coordinator")
	if err = os.Mkdir(localPath, 0744); err != nil {
		fmt.Printf("Fail to create coordinator directory, err[%s]", err.Error())
		return err
	}
	if err = os.Mkdir(filepath.Join(localPath, "log"), 0755); err != nil {
		fmt.Printf("Fail to create log directory of coordinator, err[%s]", err.Error())
		return err
	}

	if c.loggerF, err = os.OpenFile(filepath.Join(localPath, "log", "coordinator.log"),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644); err != nil {
		fmt.Printf("Fail to open log of coordinator, err[%s]", err.Error())
		return err
	}
	c.logger = log.New(c.loggerF, "", log.Lshortfile|log.LstdFlags)
	return nil
}

func (c *Coordinator) Start() error {
	std_rpc.Register(c)
	std_rpc.HandleHTTP()
	sockname := rpc.GetMasterAddress()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		c.logger.Fatal("Coordinator listen error:", e)
		return e
	}
	go http.Serve(l, nil)
	c.wait.Add(3)
	go c.startHeartbeat()
	go c.scheduleTask()
	go c.startNotify()
	return nil
}

func (c *Coordinator) destroyAllWorker() {
	for _, i := range c.workerInfoList {
		args := &rpc.DestroyArgs{
			Id: i.id,
		}
		reply := &rpc.DestroyReply{}
		method := fmt.Sprintf("%s.Destroy", c.serviceName)
		err := rpc.Call(method, i.client, args, reply)
		if err != nil || !reply.S.OK {
			c.logger.Printf("Fail to destroy worker[%s], addr[%s], err[%v], msg[%s]",
				i.id, i.addr, err, reply.S.Msg)
		}
		i.client.Close()
	}
}

func (c *Coordinator) Done() {
	c.wait.Wait()
	c.m.Lock()
	defer c.m.Unlock()
	for c.unfinishReducerTaskNum != 0 {
		time.Sleep(1 * time.Second)
	}
	c.logger.Printf("Coordinator has finished")
	c.destroyAllWorker()
	c.loggerF.Close()
}
