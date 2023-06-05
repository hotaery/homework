package common

// enum
const (
	TASK_TYPE_MAP         int = 0
	TASK_TYPE_REDUCE      int = 1
	TASK_STATE_IDLE       int = 0
	TASK_STATE_IN_PROCESS int = 1
	TASK_STATE_COMPLETED  int = 2
	WORKER_STATE_HEALTH   int = 0
	WORKER_STATE_BROKEN   int = 1
)

const (
	FileSystemRootPath string = "."
)

type TaskInfo struct {
	TaskName   string
	TaskType   int
	NReduce    int
	InputFile  []string
	OutputFile string
}
