package common

import (
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

const (
	MaxRpcRetry int = 3
)

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

// coordinator -> worker
type AssignTaskArgs struct {
	Info TaskInfo
}

type AssignTaskReply struct {
}

type StopWorkerArgs struct {
}

type StopWorkerReply struct {
}

type HeartbeatArgs struct {
}

type HeartbeatReply struct {
}

// worker -> coordinator
type RegisterWorkerArgs struct {
	Address string
}

type RegisterWorkerReply struct {
}

type CompleteTaskArgs struct {
	TaskName       string
	OutputFileList []string
	Succ           bool
	Msg            string
}

type CompleteTaskReply struct {
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func CoordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
