package worker

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"mr/rpc"
	std_rpc "net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type TestContext struct {
	tempDir string
	nMap    int
	nReduce int
	client  *std_rpc.Client
}

func (ctx *TestContext) InitInput() (err error) {
	if err = os.Mkdir(filepath.Join(ctx.tempDir, "input"), 0755); err != nil {
		return
	}

	var f *os.File
	for i := 0; i < ctx.nMap; i++ {
		fname := fmt.Sprintf("file-%d", i)
		if f, err = os.Create(filepath.Join(ctx.tempDir, "input", fname)); err != nil {
			return
		}
		f.Close()
	}

	return
}

func (ctx *TestContext) MapF(filename string, content string) []rpc.KeyValue {
	val := rand.Intn(10000)
	f, _ := os.OpenFile(filepath.Join(ctx.tempDir, "input", filename), os.O_WRONLY, 0)
	valS := strconv.Itoa(val)
	f.Write([]byte(valS))
	ans := make([]rpc.KeyValue, 0)
	ans = append(ans, rpc.KeyValue{
		Key:   filename,
		Value: valS,
	})
	return ans
}

func (ctx *TestContext) ReduceF(key string, values []string) string {
	if len(values) != 1 {
		fmt.Printf("TestContext::ReduceF: %s=>%v", key, values)
		os.Exit(1)
	}
	return values[0]
}

func TestWorker(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		t.Fatalf("MkdirTemp: %s", err.Error())
	}
	defer os.RemoveAll(tempDir)
	ctx := &TestContext{
		tempDir: tempDir,
		nMap:    rand.Intn(8) + 4,
		nReduce: 2, //rand.Intn(2) + 1,
	}
	if err = ctx.InitInput(); err != nil {
		t.Fatalf("Fail to init input file, [%s]", err.Error())
	}
	if err = os.Mkdir(filepath.Join(tempDir, "output"), 0755); err != nil {
		t.Fatalf("Fail to create output dir, err[%s]", err.Error())
	}

	w := &Worker{}
	mapF := func(filename string, content string) []rpc.KeyValue {
		return ctx.MapF(filename, content)
	}
	reduceF := func(key string, values []string) string {
		return ctx.ReduceF(key, values)
	}
	if err = w.Init(tempDir, mapF, reduceF); err != nil {
		t.Fatalf("Fail to init worker, err[%s]", err.Error())
	}
	w.Start()
	if ctx.client, err = std_rpc.DialHTTP("tcp", w.ListenAddress()); err != nil {
		t.Fatalf("Fail to connect %s, err[%s]", w.ListenAddress(), err.Error())
	}
	for i := 0; i < ctx.nMap; i++ {
		args := &rpc.AssignArgs{
			Id:          w.Id(),
			Type:        rpc.TASK_TYPE_MAP,
			TaskId:      uint64(i),
			InputPath:   filepath.Join(tempDir, "input"),
			OutputPath:  "",
			FileId:      uint64(i * ctx.nReduce),
			TotalReduce: ctx.nReduce,
			TotalMap:    ctx.nMap,
		}
		args.InputFileList = make([]string, 0)
		args.InputFileList = append(args.InputFileList, fmt.Sprintf("file-%d", i))
		reply := &rpc.AssignReply{}
		if err = rpc.Call("Worker.Assign", ctx.client, args, reply); err != nil {
			t.Fatalf("Fail to assign mapper, err[%s]", err.Error())
		}
		if !reply.S.OK {
			t.Fatalf("Fail to assign mapper in app level, err[%s]", reply.S.Msg)
		}
	}
	fmt.Println("Finish assign mapper task")
	for i := 0; i < ctx.nReduce; i++ {
		args := &rpc.AssignArgs{
			Id:          w.Id(),
			Type:        rpc.TASK_TYPE_REDUCE,
			TaskId:      uint64(i),
			InputPath:   "",
			OutputPath:  filepath.Join(tempDir, "output"),
			FileId:      uint64(i),
			TotalReduce: ctx.nReduce,
			TotalMap:    ctx.nMap,
		}
		reply := &rpc.AssignReply{}
		if err = rpc.Call("Worker.Assign", ctx.client, args, reply); err != nil {
			t.Fatalf("Fail to assign reducer, err[%s]", err.Error())
		}
		if !reply.S.OK {
			t.Fatalf("Fail to assign reducer in app level, err[%s]", reply.S.Msg)
		}
	}
	fmt.Println("Finish assign reducer task")

	err = nil
	go func() {
		fmt.Println("Start heartbeat goroutine")
		for {
			args := &rpc.HeartbeatArgs{
				Id: w.Id(),
			}
			reply := &rpc.HeartbeatReply{}
			if err = rpc.Call("Worker.Heartbeat", ctx.client, args, reply); err != nil {
				break
			}

			if !reply.S.OK {
				err = fmt.Errorf("Fail to Heartbeat[%s]", reply.S.Msg)
				break
			}

			if len(reply.TaskIdList) != ctx.nMap+ctx.nReduce {
				err = fmt.Errorf("Mismatch task length[%d:%d]", len(reply.TaskIdList), ctx.nMap+ctx.nReduce)
				break
			}
			unfinishTaskNumber := 0
			for i := 0; i < len(reply.TaskIdList); i++ {
				if reply.TaskStatusList[i] != rpc.TASK_STATUS_COMPLETE {
					unfinishTaskNumber++
				}
			}
			if unfinishTaskNumber == 0 {
				break
			}

			for i := 0; i < len(reply.TaskIdList); i++ {
				taskId := reply.TaskIdList[i]
				taskType := reply.TaskTypeList[i]
				taskStatus := reply.TaskStatusList[i]
				if taskStatus == rpc.TASK_STATUS_ERROR {
					err = fmt.Errorf("Task[%d:%d] failed", taskId, taskType)
					break
				}
				if taskType == rpc.TASK_TYPE_MAP && taskStatus == rpc.TASK_STATUS_COMPLETE {
					for j := 0; j < ctx.nReduce; j++ {
						notifyArgs := &rpc.NotifyArgs{
							Id:           w.Id(),
							Type:         rpc.TASK_TYPE_REDUCE,
							TaskId:       uint64(j),
							OldId:        "",
							NewId:        w.Id(),
							NewAddr:      w.ListenAddress(),
							NewFileId:    taskId*uint64(ctx.nReduce) + uint64(j),
							MapperTaskId: taskId,
						}
						notifyReply := &rpc.NotifyReply{}
						if err = rpc.Call("Worker.Notify", ctx.client, notifyArgs, notifyReply); err != nil {
							break
						}
						if !notifyReply.S.OK {
							err = fmt.Errorf("Fail to notify[%d], err[%s]", j, notifyReply.S.Msg)
							break
						}
					}
				}
				if err != nil {
					break
				}
			}
			if err != nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		fmt.Printf("Stop heartbeat goroutine, err[%v]\n", err)
	}()

	{
		args := &rpc.DestroyArgs{
			Id: w.Id(),
		}
		reply := &rpc.DestroyReply{}
		if err := rpc.Call("Worker.Destroy", ctx.client, args, reply); err != nil {
			t.Fatalf("Worker.Destroy, err[%s]", err.Error())
		}
		if !reply.S.OK {
			t.Fatalf("Worker.Destroy in app level, err[%s]", reply.S.Msg)
		}
		fmt.Println("Succ to destroy worker")
	}
	w.Done()

	fmt.Println("Start to check output")
	{
		outputPath := filepath.Join(tempDir, "output")
		inputPath := filepath.Join(tempDir, "input")
		children, err := os.ReadDir(outputPath)
		if err != nil {
			t.Fatalf("ReadDir, path[%s], err[%s]", outputPath, err.Error())
		}
		if len(children) != ctx.nReduce {
			t.Fatalf("Mismatch number of output file, expect[%d], actual[%d]", ctx.nReduce, len(children))
		}

		for _, child := range children {
			f, _ := os.Open(filepath.Join(outputPath, child.Name()))
			defer f.Close()
			lineF := bufio.NewReader(f)
			for {
				line, _, err := lineF.ReadLine()
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("ReadLine, file[%s], err[%s]", child.Name(), err.Error())
				}
				kv := strings.Split(string(line), " ")
				if len(kv) != 2 {
					t.Fatalf("Invalid format, line[%s]", string(line))
				}
				inputF, _ := os.Open(filepath.Join(inputPath, kv[0]))
				defer inputF.Close()
				inputByte, _ := io.ReadAll(inputF)
				expectVal, _ := strconv.Atoi(string(inputByte))
				actualVal, _ := strconv.Atoi(kv[1])
				if expectVal != actualVal {
					t.Fatalf("Mismatch output for key[%s], expect[%d], actual[%d]", kv[0], expectVal, actualVal)
				}
			}
		}
	}
}
