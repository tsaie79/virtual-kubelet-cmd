package main

import (
    "context"
    // "io"
    "os"
    "syscall"
	"github.com/containerd/fifo"
	"os/exec"
	"errors"
	"io"
	"fmt"
	"time"
)


func runCmd(cmd string) {
	time.Sleep(1 * time.Second)
	env := os.Environ()
	cmd = cmd + fmt.Sprintf(" > %s/hostpipe/stdout 2> %s/hostpipe/stderr", os.Getenv("HOME"), os.Getenv("HOME"))
	fmt.Printf("cmd: %s\n", cmd)
	cmd2 := exec.Command("/bin/bash", "-c", cmd)
	cmd2.Env = env
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	err := cmd2.Run()
	if err != nil {
		fmt.Printf("Error running command: %v\n", err)
		return
	}
}




func readCmdFromFifo() {
	fifoPath := os.Getenv("HOME") + "/hostpipe"
    ctx := context.Background()
    fn := fifoPath + "/vk-cmd"
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
			if errors.Is(err, io.EOF) {
				// EOF error, break the loop
				fmt.Println("EOF reached, breaking the loop")
				continue
			} else {
				fmt.Printf("Error reading from FIFO: %v\n", err)
				return
			}
		}

		go runCmd(string(buf[:n]))
		fifo.Close()
		fmt.Printf("Read %d bytes from FIFO: %s\n", n, buf[:n])
		// run cmd from fifo output
		// fifo.Close()
	}
}

func main() {
	readCmdFromFifo()
}
