package main

//
// start a worker process, which is implemented
// in ../mr/worker.go. typically there will be
// multiple worker processes, talking to one coordinator.
//
// go run mrworker.go wc.so
//
// Please do not change this file.
//

import (
	"fmt"
	"log"
	"mr/rpc"
	"mr/worker"
	std_rpc "net/rpc"
	"os"
	"plugin"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: mrworker xxx.so\n")
		os.Exit(1)
	}
	localPath := os.Getenv("LOCAL_PATH")
	if localPath == "" {
		localPath = "output"
	}

	mapf, reducef := loadPlugin(os.Args[1])

	w := &worker.Worker{}
	if err := w.Init(localPath, mapf, reducef); err != nil {
		fmt.Printf("Fail to init worker")
		os.Exit(1)
	}
	w.Start()
	args := rpc.RegisterArgs{}
	args.Id = w.Id()
	args.Addr = w.ListenAddress()
	reply := &rpc.RegisterReply{}
	client, err := std_rpc.DialHTTP("unix", rpc.GetMasterAddress())
	if err != nil {
		fmt.Printf("Fail to connect coordinator, err[%s], id[%s]", err.Error(), w.Id())
		os.Exit(1)
	}
	defer client.Close()
	err = rpc.Call("Coordinator.Register", client, args, reply)
	if err != nil || !reply.S.OK {
		fmt.Printf("Fail to register worker[%s], err[%v], msg[%s]", w.Id(), err, reply.S.Msg)
		os.Exit(1)
	}
	w.Done()
}

// load the application Map and Reduce functions
// from a plugin file, e.g. ../mrapps/wc.so
func loadPlugin(filename string) (func(string, string) []rpc.KeyValue, func(string, []string) string) {
	p, err := plugin.Open(filename)
	if err != nil {
		log.Fatalf("cannot load plugin %v", filename)
	}
	xmapf, err := p.Lookup("Map")
	if err != nil {
		log.Fatalf("cannot find Map in %v", filename)
	}
	mapf := xmapf.(func(string, string) []rpc.KeyValue)
	xreducef, err := p.Lookup("Reduce")
	if err != nil {
		log.Fatalf("cannot find Reduce in %v", filename)
	}
	reducef := xreducef.(func(string, []string) string)

	return mapf, reducef
}
