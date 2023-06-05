package coordinator

import (
	"errors"
	"fmt"
	"log"
	"math"
	"mr/common"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"sync"
	"time"
)

type taskStatus struct {
	info           common.TaskInfo
	state          int
	canSchedule    bool
	worker         string
	outputFileList []string
}

type workerStatus struct {
	address     string
	workerState int
	task        map[string]*taskStatus
}

type Coordinator struct {
	mutex              sync.Mutex
	workerMap          map[string]*workerStatus
	inputFileList      []string
	finished           bool
	nReduce            int
	taskMap            map[string]*taskStatus
	maxTaskNumOfWorker int
	mapTaskNum         int
	reduceTaskNum      int
	heartbeatCnt       int
	heartbeatCh        chan int
}

func call(name string, address string, args interface{}, reply interface{}) error {
	c, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Call(name, args, reply)
}

// mutex held
func (c *Coordinator) selectWorker() string {
	var addr string
	minTaskNum := math.MaxInt32
	for _, v := range c.workerMap {
		if len(v.task) >= c.maxTaskNumOfWorker {
			continue
		}
		if v.workerState != common.WORKER_STATE_HEALTH {
			continue
		}
		if len(v.task) < minTaskNum {
			addr = v.address
			minTaskNum = len(v.task)
		}
	}
	return addr
}

func (c *Coordinator) scheduleTaskTimer() {
	for {
		if c.scheduleTask() {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func (c *Coordinator) scheduleTask() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.finished {
		return true
	}

	taskList := make([]string, 0)
	addressList := make([]string, 0)
	for _, v := range c.taskMap {
		if v.canSchedule && v.state == common.TASK_STATE_IDLE {
			address := c.selectWorker()
			if len(address) == 0 {
				break
			}
			v.state = common.TASK_STATE_IN_PROCESS
			v.worker = address
			taskList = append(taskList, v.info.TaskName)
			addressList = append(addressList, address)
			c.workerMap[address].task[v.info.TaskName] = v
		}
	}

	log.Printf("start to assign %d task\n", len(taskList))

	argsList := make([]*common.AssignTaskArgs, 0)
	replyList := make([]*common.AssignTaskReply, 0)
	for i := 0; i < len(taskList); i++ {
		args := &common.AssignTaskArgs{}
		args.Info = c.taskMap[taskList[i]].info
		argsList = append(argsList, args)
		replyList = append(replyList, &common.AssignTaskReply{})
	}

	assignTaskClosure := func(idx int) {
		succ := false
		for i := 0; i < common.MaxRpcRetry; i++ {
			err := call("WorkerService.AssignTask", addressList[idx], argsList[idx], replyList[idx])
			if err != nil {
				log.Printf("fail to assign task %s to worker %s, error: %v, retry: %d\n",
					taskList[idx], addressList[idx], err, i)
			} else {
				log.Printf("succ to assign task %s to worker %s, retry: %d\n", taskList[idx], addressList[idx], i)
				succ = true
				break
			}
		}

		c.mutex.Lock()
		defer c.mutex.Unlock()
		if !succ {
			log.Printf("fail to assign task %s, set state to idle", taskList[idx])
			c.taskMap[taskList[idx]].state = common.TASK_STATE_IDLE
			c.taskMap[taskList[idx]].worker = ""
			delete(c.workerMap[addressList[idx]].task, taskList[idx])
		}
	}

	for i := 0; i < len(argsList); i++ {
		go assignTaskClosure(i)
	}
	return false
}

func (c *Coordinator) registerWorker(args *common.RegisterWorkerArgs, reply *common.RegisterWorkerReply) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.finished {
		return errors.New("mapreduce has finished")
	}
	address := args.Address
	_, ok := c.workerMap[address]
	if ok {
		log.Printf("worker %s has registered, maybe restart quickly after crashed", address)
		for _, v := range c.workerMap[address].task {
			v.state = common.TASK_STATE_IDLE
			v.worker = ""
			log.Printf("worker %s registered, move task %s to pending queue", address, v.info.TaskName)
		}
		delete(c.workerMap, address)
	}
	c.workerMap[address] = &workerStatus{
		address:     address,
		workerState: common.WORKER_STATE_HEALTH,
		task:        make(map[string]*taskStatus),
	}
	log.Printf("register worker %s\n", address)
	return nil
}

func (c *Coordinator) completeTask(args *common.CompleteTaskArgs, reply *common.CompleteTaskReply) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if _, ok := c.taskMap[args.TaskName]; !ok {
		log.Printf("recv task %s which belong to this mapreduce", args.TaskName)
		return nil
	}
	state := c.taskMap[args.TaskName].state
	if state == common.TASK_STATE_COMPLETED {
		log.Printf("recv duplicate complete task request %s", args.TaskName)
		return nil
	}
	if !args.Succ {
		log.Printf("fail to excute task %s, error: %s, state: %d\n", args.TaskName, args.Msg, state)
		delete(c.workerMap[c.taskMap[args.TaskName].worker].task, args.TaskName)
		c.taskMap[args.TaskName].state = common.TASK_STATE_IDLE
		c.taskMap[args.TaskName].worker = ""
	} else {
		log.Printf("succ to complete task %s on worker %s, output: %s, state: %d\n",
			args.TaskName, c.taskMap[args.TaskName].worker, strings.Join(args.OutputFileList, ","), state)
		c.taskMap[args.TaskName].state = common.TASK_STATE_COMPLETED
		c.taskMap[args.TaskName].outputFileList = args.OutputFileList
		delete(c.workerMap[c.taskMap[args.TaskName].worker].task, args.TaskName)
		if c.taskMap[args.TaskName].info.TaskType == common.TASK_TYPE_MAP {
			c.mapTaskNum--
		} else {
			c.reduceTaskNum--
		}
		if c.mapTaskNum == 0 {
			for _, v := range c.taskMap {
				if v.info.TaskType == common.TASK_TYPE_REDUCE && !v.canSchedule {
					log.Printf("map tasks have completed, this point can shedule reduce task %s", v.info.TaskName)
					v.canSchedule = true
				}
			}
			if c.reduceTaskNum == 0 {
				log.Printf("reduce tasks have completed, this point can finish mapreduce job")
				c.finished = true
			}
		}
	}
	return nil
}

func (c *Coordinator) heartbeatTimer() {
	for {
		workerList := make([]string, 0)
		c.mutex.Lock()
		if c.finished {
			c.mutex.Unlock()
			break
		}
		if c.heartbeatCnt > 0 {
			c.mutex.Unlock()
			time.Sleep(1 * time.Second)
			continue
		}
		for _, v := range c.workerMap {
			if len(v.task) > 0 {
				workerList = append(workerList, v.address)
			}
		}
		c.heartbeatCnt = len(workerList)
		c.mutex.Unlock()

		callHeartbeat := func(idx int) {
			args := &common.HeartbeatArgs{}
			reply := &common.HeartbeatReply{}
			succ := false
			for j := 0; j < common.MaxRpcRetry; j++ {
				err := call("WorkerService.Heartbeat", workerList[idx], &args, &reply)
				if err != nil {
					log.Printf("fail to heartbeat to %s, err: %v\n", workerList[idx], err)
				} else {
					succ = true
					break
				}
			}
			c.mutex.Lock()
			defer c.mutex.Unlock()
			c.heartbeatCnt--
			if !succ {
				for k := range c.workerMap[workerList[idx]].task {
					c.taskMap[k].state = common.TASK_STATE_IDLE
					c.taskMap[k].worker = ""
					log.Printf("worker %s has broken, move task %s to pending queue\n", workerList[idx], k)
				}
				delete(c.workerMap, workerList[idx])
			} else {
				log.Printf("worker %s is alive, task %d", workerList[idx], len(c.workerMap[workerList[idx]].task))
			}
		}
		for i := 0; i < len(workerList); i++ {
			go callHeartbeat(i)
		}

		time.Sleep(1 * time.Second)
	}
	c.heartbeatCh <- 0
}

func (c *Coordinator) init(files []string) error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	logFile, _ := os.OpenFile("log/coordinator.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	log.SetOutput(logFile)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.inputFileList = files
	for i := 0; i < len(files); i++ {
		taskName := fmt.Sprintf("map-%d", i)
		c.taskMap[taskName] = &taskStatus{
			info: common.TaskInfo{
				TaskName:   taskName,
				TaskType:   common.TASK_TYPE_MAP,
				NReduce:    c.nReduce,
				InputFile:  []string{files[i]},
				OutputFile: taskName,
			},
			state:       common.TASK_STATE_IDLE,
			canSchedule: true,
		}
	}
	c.mapTaskNum = len(files)
	for i := 0; i < c.nReduce; i++ {
		taskName := fmt.Sprintf("reduce-%d", i)
		c.taskMap[taskName] = &taskStatus{
			info: common.TaskInfo{
				TaskName:   taskName,
				TaskType:   common.TASK_TYPE_REDUCE,
				NReduce:    0,
				OutputFile: fmt.Sprintf("mr-out-%d", i),
			},
			state:       common.TASK_STATE_IDLE,
			canSchedule: false,
		}
		for j := 0; j < len(files); j++ {
			c.taskMap[taskName].info.InputFile = append(c.taskMap[taskName].info.InputFile, fmt.Sprintf("map-%d-%d", j, i))
		}
	}
	c.reduceTaskNum = c.nReduce
	log.Printf("start to exec mapreduce, map: %d, reduce: %d\n", c.mapTaskNum, c.reduceTaskNum)
	go c.heartbeatTimer()
	go c.scheduleTaskTimer()
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	svc := &CoordinatorService{
		c: c,
	}
	rpc.Register(svc)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := common.CoordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

func (c *Coordinator) notifyWorker() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, v := range c.workerMap {
		args := &common.StopWorkerArgs{}
		reply := &common.StopWorkerReply{}
		for i := 0; i < common.MaxRpcRetry; i++ {
			err := call("WorkerService.Stop", v.address, args, reply)
			if err != nil {
				log.Printf("fail to stop worker %s, err: %v\n", v.address, err)
			} else {
				log.Printf("succ to stop worker %s\n", v.address)
				break
			}
		}
	}
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	<-c.heartbeatCh
	log.Printf("mapreduce has finished")
	c.notifyWorker()
	return c.finished
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		nReduce:            nReduce,
		finished:           false,
		taskMap:            make(map[string]*taskStatus),
		workerMap:          make(map[string]*workerStatus),
		mapTaskNum:         0,
		reduceTaskNum:      0,
		maxTaskNumOfWorker: 1,
		heartbeatCnt:       0,
		heartbeatCh:        make(chan int),
	}
	c.init(files)
	c.server()
	return &c
}
