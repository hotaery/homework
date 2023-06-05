package worker

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mr/common"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type taskStatus struct {
	taskInfo       common.TaskInfo
	taskState      int
	outputFileList []string
	ch             chan int
	err            error
}

type taskManager struct {
	mu                sync.Mutex
	mapf              func(string, string) []KeyValue
	reducef           func(string, []string) string
	fs                common.FileSystem
	taskMap           map[string]*taskStatus
	maxRunningTaskNum int
	runningTaskNum    int
	stopped           bool
}

func (tm *taskManager) init(maxRunningTaskNum int) error {
	tm.maxRunningTaskNum = maxRunningTaskNum
	return nil
}

func (tm *taskManager) stop() {
	tm.mu.Lock()
	tm.stopped = true
	tm.mu.Unlock()

	for _, v := range tm.taskMap {
		<-v.ch
	}
	log.Println("taskManager has stopped")
}

func (tm *taskManager) join() {
	for {
		tm.mu.Lock()
		stop := tm.stopped
		tm.mu.Unlock()
		if stop {
			break 
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}

func (tm *taskManager) notifyCoordinator(task *taskStatus) {
	args := &common.CompleteTaskArgs {
		TaskName: task.taskInfo.TaskName,
		OutputFileList: task.outputFileList,
		Succ: true,
	}
	if task.err != nil {
		args.Succ = false
		args.Msg = task.err.Error()
	}
	reply := &common.CompleteTaskReply{}
	for i := 0; i < common.MaxRpcRetry; i++ {
		err := call("CoordinatorService.CompleteTask", args, reply)
		if err != nil {
			log.Printf("fail to call CoordinatorService.CompleteTask, error: %v, retry: %d\n", err, i)
		} else {
			break;
		}
	}
	// TODO: if coordinator failed, how to process task result?
	log.Printf("task %s has finished, output: %s", task.taskInfo.TaskName, strings.Join(task.outputFileList, ","))
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.taskMap, task.taskInfo.TaskName)
}

func (tm *taskManager) assignTask(task common.TaskInfo) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.stopped {
		return
	}
	_, ok := tm.taskMap[task.TaskName]
	if ok {
		return
	}
	tm.taskMap[task.TaskName] = &taskStatus{
		taskInfo:  task,
		taskState: common.TASK_STATE_IDLE,
		ch: make(chan int),
	}
	if tm.runningTaskNum < tm.maxRunningTaskNum {
		tm.taskMap[task.TaskName].taskState = common.TASK_STATE_IN_PROCESS
		tm.runningTaskNum++
		go tm.start(tm.taskMap[task.TaskName])
	}
}

func (tm *taskManager) start(task *taskStatus) {
	log.Printf("start task, task name: %s, task type: %d, task input: %s, reduce: %d",
		task.taskInfo.TaskName, task.taskInfo.TaskType, strings.Join(task.taskInfo.InputFile, ","), task.taskInfo.NReduce)
	if task.taskInfo.TaskType == common.TASK_TYPE_MAP {
		tm.startMapTask(task)
	} else if task.taskInfo.TaskType == common.TASK_TYPE_REDUCE {
		tm.startReduceTask(task)
	} else {
		msg := fmt.Sprintf("unkonown task type: %d, task name: %s", task.taskInfo.TaskType, task.taskInfo.TaskName)
		log.Println(msg)
		task.ch <- 0
	}
	tm.onTaskFinish(task)
}

func (tm *taskManager) startMapTask(task *taskStatus) {
	defer tm.notifyCoordinator(task)
	if len(task.taskInfo.InputFile) != 1 {
		msg := fmt.Sprintf("map task has multi input file: %s", strings.Join(task.taskInfo.InputFile, ","))
		log.Println(msg)
		task.err = errors.New(msg)
		return
	}
	file, err := tm.fs.Open(task.taskInfo.InputFile[0], os.O_RDONLY, 0)
	if err != nil {
		log.Printf("fail to open file: %s, err: %v\n", task.taskInfo.InputFile, err)
		task.err = err
		return
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("fail to read file: %s, err: %v\n", task.taskInfo.InputFile, err)
		task.err = err
		return
	}
	mapStartTime := time.Now()
	kvList := tm.mapf(task.taskInfo.InputFile[0], string(content))
	mapelapsedTime := time.Since(mapStartTime)
	log.Printf("task %s duration ms: %d", task.taskInfo.TaskName, mapelapsedTime.Milliseconds())
	outfileList := make([]common.File, 0)
	for i := 0; i < task.taskInfo.NReduce; i++ {
		ofname := fmt.Sprintf("%s-%d", task.taskInfo.OutputFile, i)
		of, err := tm.fs.Open(ofname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("fail to open %s, err: %v\n", ofname, err)
			task.err = err
			return
		}
		defer of.Close()
		outfileList = append(outfileList, of)
		task.outputFileList = append(task.outputFileList, ofname)
	}
	log.Printf("task %s generate %d kv", task.taskInfo.TaskName, len(kvList))
	for _, kv := range kvList {
		msg := fmt.Sprintf("%s %s\n", kv.Key, kv.Value)
		idx := ihash(kv.Key) % task.taskInfo.NReduce
		n, err := outfileList[idx].Append([]byte(msg))
		if err != nil {
			log.Printf("fail to write %s, err: %v\n", outfileList[idx].Name(), err)
			task.err = err
			return
		}
		if n != len(msg) {
			msg := fmt.Sprintf("insufficient bytes when write file %s, written: %d, expect: %d",
				outfileList[idx].Name(), n, len(msg))
			log.Println(msg)
			task.err = errors.New(msg)
			return
		}
	}
}

func (tm *taskManager) startReduceTask(task *taskStatus) {
	defer tm.notifyCoordinator(task)
	tmpFile := fmt.Sprintf("%s.tmp", task.taskInfo.OutputFile)
	ofile, err := tm.fs.Open(tmpFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("fail to open %s, err: %v\n", tmpFile, err)
		task.err = err
		return
	}
	defer ofile.Close()
	defer tm.fs.Unlink(tmpFile)
	intermediateKV := []KeyValue{}
	for _, fname := range task.taskInfo.InputFile {
		file, err := tm.fs.Open(fname, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("fail to open %s, err: %v\n", fname, err)
			task.err = err
			return
		}
		defer file.Close()
		content, err := ioutil.ReadAll(file)
		if err != nil {
			log.Printf("fail to read %s, err: %v\n", fname, err)
			task.err = err
			return
		}
		kvList := strings.Split(string(content), "\n")
		for i, kv := range kvList {
			if len(kv) == 0 {
				continue
			}
			kvp := strings.Split(kv, " ")
			if len(kvp) != 2 {
				msg := fmt.Sprintf("invalid format kv: %s, file: %s, idx: %d", kv, fname, i)
				log.Println(msg)
				task.err = errors.New(msg)
				return
			}
			internalKV := KeyValue{
				Key:   kvp[0],
				Value: kvp[1],
			}
			intermediateKV = append(intermediateKV, internalKV)
		}
	}

	log.Printf("reduce task %s has %d record\n", task.taskInfo.TaskName, len(intermediateKV))
	sort.Sort(ByKey(intermediateKV))
	i := 0
	for i < len(intermediateKV) {
		j := i + 1
		reduceInput := []string{}
		reduceInput = append(reduceInput, intermediateKV[i].Value)
		for j < len(intermediateKV) {
			if intermediateKV[j].Key == intermediateKV[j-1].Key {
				reduceInput = append(reduceInput, intermediateKV[j].Value)
				j++
			} else {
				break
			}
		}
		reduceOutput := tm.reducef(intermediateKV[i].Key, reduceInput)
		serializeStr := fmt.Sprintf("%s %s", intermediateKV[i].Key, reduceOutput)
		if j != len(intermediateKV) {
			serializeStr = fmt.Sprintf("%s\n", serializeStr)
		}
		n, err := ofile.Append([]byte(serializeStr))
		if err != nil {
			log.Printf("fail to write %s, err: %v\n", tmpFile, err)
			task.err = err
			return
		}
		if n != len(serializeStr) {
			msg := fmt.Sprintf("insufficient bytes when write %s, written: %d, expect: %d",
				tmpFile, n, len(serializeStr))
			log.Println(msg)
			task.err = errors.New(msg)
			return
		}
		i = j
	}
	_, err = ofile.Append([]byte("\n"))
	if err != nil {
		log.Printf("fail to write new line, file: %s\n", tmpFile)
		task.err = err
		return
	}

	err = tm.fs.Rename(tmpFile, task.taskInfo.OutputFile)
	if err != nil {
		log.Printf("fail to rename %s to %s, err: %v\n", tmpFile, task.taskInfo.OutputFile, err)
		task.err = err
	}
	task.outputFileList = append(task.outputFileList, task.taskInfo.OutputFile)
}

func (tm *taskManager) onTaskFinish(task *taskStatus) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.runningTaskNum--
	delete(tm.taskMap, task.taskInfo.TaskName)
	for _, v := range tm.taskMap {
		if v.taskState == common.TASK_STATE_IDLE && tm.runningTaskNum < tm.maxRunningTaskNum {
			tm.runningTaskNum++
			v.taskState = common.TASK_STATE_IN_PROCESS
			go tm.start(v)
			if tm.runningTaskNum >= tm.maxRunningTaskNum {
				break
			}
		}
	}
}
