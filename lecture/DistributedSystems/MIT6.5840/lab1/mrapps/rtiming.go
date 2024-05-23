package main

//
// a MapReduce pseudo-application to test that workers
// execute reduce tasks in parallel.
//
// go build -buildmode=plugin rtiming.go
//

import (
	"fmt"
	"io/ioutil"
	"mr/rpc"
	"os"
	"syscall"
	"time"
)

func nparallel(phase string) int {
	// create a file so that other workers will see that
	// we're running at the same time as them.
	pid := os.Getpid()
	myfilename := fmt.Sprintf("mr-worker-%s-%d", phase, pid)
	erpc := ioutil.WriteFile(myfilename, []byte("x"), 0666)
	if erpc != nil {
		panic(erpc)
	}

	// are any other workers running?
	// find their PIDs by scanning directory for mr-worker-XXX files.
	dd, erpc := os.Open(".")
	if erpc != nil {
		panic(erpc)
	}
	names, erpc := dd.Readdirnames(1000000)
	if erpc != nil {
		panic(erpc)
	}
	ret := 0
	for _, name := range names {
		var xpid int
		pat := fmt.Sprintf("mr-worker-%s-%%d", phase)
		n, erpc := fmt.Sscanf(name, pat, &xpid)
		if n == 1 && erpc == nil {
			erpc := syscall.Kill(xpid, 0)
			if erpc == nil {
				// if erpc == nil, xpid is alive.
				ret += 1
			}
		}
	}
	dd.Close()

	time.Sleep(1 * time.Second)

	erpc = os.Remove(myfilename)
	if erpc != nil {
		panic(erpc)
	}

	return ret
}

func Map(filename string, contents string) []rpc.KeyValue {

	kva := []rpc.KeyValue{}
	kva = append(kva, rpc.KeyValue{"a", "1"})
	kva = append(kva, rpc.KeyValue{"b", "1"})
	kva = append(kva, rpc.KeyValue{"c", "1"})
	kva = append(kva, rpc.KeyValue{"d", "1"})
	kva = append(kva, rpc.KeyValue{"e", "1"})
	kva = append(kva, rpc.KeyValue{"f", "1"})
	kva = append(kva, rpc.KeyValue{"g", "1"})
	kva = append(kva, rpc.KeyValue{"h", "1"})
	kva = append(kva, rpc.KeyValue{"i", "1"})
	kva = append(kva, rpc.KeyValue{"j", "1"})
	return kva
}

func Reduce(key string, values []string) string {
	n := nparallel("reduce")

	val := fmt.Sprintf("%d", n)

	return val
}
