package main

import (
	"fmt"
	"mr/coordinator"
	"os"
	"path/filepath"
)

func main() {
	localPath := os.Getenv("LOCAL_PATH")
	if len(localPath) == 0 {
		localPath = "output"
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: <input file list>")
		os.Exit(1)
	}

	if err := os.Mkdir(filepath.Join(localPath, "result"), 0755); err != nil {
		fmt.Printf("Fail to create output directory, err[%s]", err.Error())
		os.Exit(1)
	}

	c := &coordinator.Coordinator{}
	options := coordinator.CoordinatorOptions{
		LocalPath:         localPath,
		InputPath:         ".", // TODO
		InputFileList:     os.Args[1:],
		OutputPath:        filepath.Join(localPath, "result"),
		ServiceName:       "Worker",
		MaxNumberOfWorker: 1, // for mtiming.go, one task one worker
		NReduce:           8,
		Networking:        "tcp",
	}
	if err := c.Init(options); err != nil {
		fmt.Printf("Fail to init coordinator, err[%s]", err.Error())
		os.Exit(1)
	}

	if err := c.Start(); err != nil {
		fmt.Printf("Fail to start coordinator, err[%s]", err.Error())
		os.Exit(1)
	}

	c.Done()
}
