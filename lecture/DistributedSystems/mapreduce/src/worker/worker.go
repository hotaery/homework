package worker

import (
	"fmt"
	"hash/fnv"
	"log"
	"mr/common"
	"net"
	"net/http"
	"net/rpc"
	"os"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

type ByKey []KeyValue

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	fs, err := common.GetLocalFileSystem(common.FileSystemRootPath)
	if err != nil {
		log.Printf("fail to init file system: %v\n", err)
		return
	}
	tm := &taskManager{
		mapf:    mapf,
		reducef: reducef,
		fs:      fs,
		taskMap: make(map[string]*taskStatus),
	}
	err = tm.init(2)
	if err != nil {
		log.Printf("fail to init taskManager: %v\n", err)
		return
	}

	svc := &WorkerService{
		tm: tm,
	}

	rpc.Register(svc)
	rpc.HandleHTTP()
	port := 8080
	for {
		l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if e != nil {
			//log.Printf("fail to listen on %d, err: %v\n", port, e)
		} else {
			go http.Serve(l, nil)
			break
		}
		port++
	}
	succ := false
	logFile, _ := os.OpenFile(fmt.Sprintf("log/worker-%d.log", port), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	log.SetOutput(logFile)
	for i := 0; i < common.MaxRpcRetry; i++ {
		args := &common.RegisterWorkerArgs{
			Address: fmt.Sprintf("127.0.0.1:%d", port),
		}
		reply := &common.RegisterWorkerReply{}
		err := call("CoordinatorService.RegisterWorker", args, reply)
		if err != nil {
			log.Printf("fail to call Coordinator.RegisterWorker, err: %v, retry: %d\n", err, i)
		} else {
			succ = true
			break
		}
	}
	if !succ {
		log.Printf("fail to register worker, worker will exit\n")
		tm.stop()
	}
	tm.join()
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) error {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := common.CoordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	return err
}
