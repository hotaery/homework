package rpc

import (
	"net/rpc"
	"os"
	"strconv"
)

const (
	RPC_MAX_RETRIES = 3
)

type TaskType int

const (
	TASK_TYPE_MAP TaskType = iota
	TASK_TYPE_REDUCE
)

type TaskStatus int

const (
	TASK_STATUS_IDLE TaskStatus = iota
	TASK_STATUS_IN_PROGRESS
	TASK_STATUS_ERROR
	TASK_STATUS_COMPLETE
)

type KeyValue struct {
	Key   string
	Value string
}

type Status struct {
	OK  bool
	Msg string
}

// worker
type AssignArgs struct {
	Id            string   // worker id
	Type          TaskType // task type
	InputFileList []string // input file name of mapper
	TaskId        uint64   // <Type, TaskId> determine a mapper or reducer uniquly
	InputPath     string   // input path in GFS
	OutputPath    string   // output path in GFS
	FileId        uint64   // generate output file name
	TotalReduce   int      // Number of output file
	TotalMap      int      // number of mapper task
}

type AssignReply struct {
	S Status
}

type HeartbeatArgs struct {
	Id string
}

type HeartbeatReply struct {
	S              Status
	TaskTypeList   []TaskType
	TaskIdList     []uint64
	TaskStatusList []TaskStatus
}

type ReadArgs struct {
	Id      string
	FileId  uint64
	LasyKey string
	Offset  int // -1 means return next(LastKey) else return No.(offset + 1) for LastKey
	N       int // Maximum KeyValue
}

type ReadReply struct {
	S            Status
	KeyValueList []KeyValue
}

type NotifyArgs struct {
	Id           string
	Type         TaskType
	TaskId       uint64
	OldId        string
	NewId        string
	NewAddr      string
	NewFileId    uint64
	MapperTaskId uint64
}

type NotifyReply struct {
	S Status
}

type DestroyArgs struct {
	Id string
}

type DestroyReply struct {
	S Status
}

// master
type RegisterArgs struct {
	Id   string
	Addr string
}

type RegisterReply struct {
	S Status
}

func Call(method string, client *rpc.Client, args interface{}, reply interface{}) (err error) {
	for i := 0; i < RPC_MAX_RETRIES; i++ {
		err = client.Call(method, args, reply)
		if err == nil {
			break
		}
	}
	return
}

func GetMasterAddress() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
