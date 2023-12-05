package main

import (
    "context"
    "fmt"
    // "io"
    "os"
    "syscall"
	"github.com/containerd/fifo"
	"os/exec"
)

func readCmdFromFifo() {
    fifoPath := "/workspaces/virtual-kubelet-cmd/fifo/hostpipe"
    ctx := context.Background()
    fn := fifoPath + "/myFifo"
    flag := syscall.O_RDONLY | syscall.O_CREAT 
    perm := os.FileMode(0666)


	for {
		fifo, err := fifo.OpenFifo(ctx, fn, flag, perm)
		if err != nil {
			fmt.Printf("Error opening FIFO: %v\n", err)
			return
		}
		// defer fifo.Close()

		buf := make([]byte, 1024)
		n, err := fifo.Read(buf)
		if err != nil {
			fmt.Printf("Error reading from FIFO: %v\n", err)
			return
		}
		fmt.Printf("Read %d bytes from FIFO: %s\n", n, buf[:n])

		// run cmd from fifo output
		read_cmd := string(buf[:n])
		env := os.Environ()
		cmd := exec.Command(read_cmd)
		cmd.Env = env

		// cmd.Env = append(cmd.Env, "message=from fifo")
		// cmd.Env = append(cmd.Env, "message2=from fifo2")

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		
		if err != nil {
			fmt.Printf("Error running command: %v\n", err)
			return
		}

		//write stdout and stderr to files via 


		// get pid of cmd
		fmt.Printf("PID: %d\n", cmd.Process.Pid)
		fifo.Close()
	}
}

func main() {
	readCmdFromFifo()
}
