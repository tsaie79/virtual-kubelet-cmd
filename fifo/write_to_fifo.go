package main

import (
    "context"
    "fmt"
    "os"
    "syscall"
	"github.com/containerd/fifo"
    "strings"
)


func writeCmdToFifo(command []string, args []string, env map[string]interface{}) error {
	homeDir := os.Getenv("HOME")
	fifoPath := homeDir + "/hostpipe"
    ctx := context.Background()
    fn := fifoPath + "/vk-cmd"
    flag := syscall.O_WRONLY 
    perm := os.FileMode(0666)

    fifo, err := fifo.OpenFifo(ctx, fn, flag, perm)
    if err != nil {
        fmt.Printf("Error opening FIFO: %v\n", err)
        return err
    }

    // write env to a single string like "export key1='value1'&& export key2='value2'..."
    var envString string
    for key, value := range env {
        // if type of value is string, use single quotes to prevent shell from interpreting the value
        switch value.(type) {
        case string:
            envString += "export " + key + "=\"" + value.(string) + "\" && "
        //if type of value is int or float, no quotes are needed
        case int:
            envString += "export " + key + "=" + fmt.Sprintf("%d", value.(int)) + " && "
        case float64:
            envString += "export " + key + "=" + fmt.Sprintf("%f", value.(float64)) + " && "
        }   
    }


    cmdString := strings.Join(command, " ")
    argsString := strings.Join(args, " ")
    //use single quotes to around the argsString to prevent shell from interpreting the args
    cmd := cmdString + " '" + envString + argsString + "'"

    fmt.Printf("cmd: %s\n", cmd)

    _, err = fifo.Write([]byte(cmd))
    if err != nil {
        fmt.Printf("Error writing to FIFO: %v\n", err)
        return err
    }

    return nil
}


func main() {
    cmds := []string{"/bin/bash", "-c"}
    args := []string{"bash /workspaces/virtual-kubelet-cmd/fifo/script.sh"}
    env := map[string]interface{}{"cpu": "4", "timeout": "60s"}
    // env := map[string]interface{}{}
    err := writeCmdToFifo(cmds, args, env)
    if err != nil {
        fmt.Printf("Error writing to FIFO: %v\n", err)
    }
}
