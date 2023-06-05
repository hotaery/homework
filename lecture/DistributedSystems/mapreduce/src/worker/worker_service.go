package worker

import "mr/common"

type WorkerService struct {
	tm *taskManager
}

func (svc *WorkerService) AssignTask(args *common.AssignTaskArgs, reply *common.AssignTaskReply) error {
	svc.tm.assignTask(args.Info)
	return nil
}

func (svc *WorkerService) Stop(args *common.StopWorkerArgs, reply *common.StopWorkerReply) error {
	svc.tm.stop()
	return nil
}

// TODO: report task statistics
func (svc *WorkerService) Heartbeat(args *common.HeartbeatArgs, reply *common.HeartbeatReply) error {
	return nil
}
