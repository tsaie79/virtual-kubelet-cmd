package main

import (
    "context"
    "os"
    "syscall"
	"github.com/containerd/fifo"
	"os/exec"
	"errors"
	"io"
	"fmt"
	"time"
	"text/tabwriter"
	"log"	
	"github.com/containers/psgo"
	"strings"
)


func runCmd(cmd string) {
	cmd = cmd + fmt.Sprintf(" >> %s/hostpipe/stdout 2>> %s/hostpipe/stderr", os.Getenv("HOME"), os.Getenv("HOME"))
	// replace ~ as prfix with $HOME
	cmd = strings.Replace(cmd, "~", os.Getenv("HOME"), 1)
	log.Printf("Running cmd: %s\n", cmd)
	cmd2 := exec.Command("/bin/bash", "-c", cmd)
	// // continue to use the same stdout and stderr
	// cmd2.Stdout = os.Stdout
	// cmd2.Stderr = os.Stderr
	// log.Printf("Stdout: %s\n", cmd2.Stdout)
	// log.Printf("Stderr: %s\n", cmd2.Stderr)
	// // write to files instead of stdout and stderr

	// print leader pid
	err := cmd2.Start()
	if err != nil {
		log.Panic(err)
		return
	}

	// get pids from the pgid of the leader process
	pgid, err := syscall.Getpgid(cmd2.Process.Pid)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("PGID: %d\n", pgid)

	// write pgid to a file $HOME/pdid.out
	pgidFile, err := os.Create(os.Getenv("HOME") + "/pgid.out")
	if err != nil {
		log.Fatal(err)
		return
	}
	defer pgidFile.Close()
	pgidFile.WriteString(fmt.Sprintf("%d", pgid))

	time.Sleep(10 * time.Second)
	cmd3 := exec.Command("/bin/sh", "-c", fmt.Sprintf("pgrep -g %d", pgid))

	out, err := cmd3.Output()
	if err != nil {
		log.Fatal(err)
		return
	}

	targetPids := strings.Split(string(out), "\n")
	// remove the last empty string
	targetPids = targetPids[:len(targetPids)-1]
	log.Printf("Child pids: %s\n", targetPids)
	getPsOutput(targetPids)
}



func getPsOutput(pids []string) {
	data, err := psgo.ProcessInfoByPids(pids, []string{"user", "pid", "ppid", "pgid", "pcpu", "comm"})
	// show those processes in with the state of "R" (running)

	if err != nil {
		log.Fatal(err)
	}
	tw := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
	for _, d := range data {
		fmt.Fprintln(tw, strings.Join(d, "\t"))
	}
	tw.Flush()
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
			// fmt.Printf("Error opening FIFO: %v\n", err)
			log.Panic(err)
			return
		}
		// defer fifo.Close()
		buf := make([]byte, 1024)
		n, err := fifo.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// EOF error, break the loop
				// fmt.Println("EOF reached, breaking the loop")
				log.Println("EOF reached, breaking the loop")
				continue
			} else {
				log.Panic(err)
				return
			}
		}
		// fmt.Printf("Read %d bytes from FIFO: %s\n", n, buf[:n])
		log.Printf("Read %d bytes from FIFO: %s\n", n, buf[:n])

		go runCmd(string(buf[:n]))
		time.Sleep(1 * time.Second)
		fifo.Close()

	}
}

func main() {
	readCmdFromFifo()
}
