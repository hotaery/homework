package coordinator

import (
	"log"
	"mr/common"
)

type CoordinatorService struct {
	c *Coordinator
}

func (svc *CoordinatorService) RegisterWorker(args *common.RegisterWorkerArgs, reply *common.RegisterWorkerReply) error {
	return svc.c.registerWorker(args, reply)
}

func (svc *CoordinatorService) CompleteTask(args *common.CompleteTaskArgs, reply *common.CompleteTaskReply) error {
	log.Printf("recv complete task %s", args.TaskName)
	return svc.c.completeTask(args, reply)
}
