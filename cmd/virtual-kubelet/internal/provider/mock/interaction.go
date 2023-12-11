// +build ignore

// This script is to add the functions to interact with the running shell processes launched by the pods.
// Functions include: delete and monitor the running shell processes. 
// The id of the running shell processes is the pgid of the process group, and we can use pgrep -g to get the pids of the processes in the group. 
// The monitor function should looks like ps command, and it should be able to show the status of the running shell processes. 
// Consider using psgo (github.com/containers/psgo) to implement the monitor function.

// ignore this file for now
package mock

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	psgo "github.com/containers/psgo"
)

// DeleteProcess deletes a process with the given pgid.
func DeleteProcess(pgid string) error {
	cmd := exec.Command("kill", "-9", pgid)
	return cmd.Run()
}

// MonitorProcesses uses the psgo library to list the processes in the given process group.
func MonitorProcesses(pgid string) ([]string, error) {
	// Get the list of PIDs in the process group
	cmd := exec.Command("pgrep", "-g", pgid)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Split the output into individual PIDs
	pids := strings.Split(string(output), "\n")

	// Use psgo to get process info for each PID
	var processes []string
	for _, pid := range pids {
		info, err := psgo.JoinNamespaceAndProcessInfo(pid, []string{"pid", "comm", "state"})
		if err != nil {
			return nil, err
		}
		processes = append(processes, info...)
	}

	return processes, nil
}

func main() {
	// Example usage
	err := DeleteProcess("1234")
	if err != nil {
		fmt.Println("Error deleting process:", err)
	}

	processes, err := MonitorProcesses("1234")
	if err != nil {
		fmt.Println("Error monitoring processes:", err)
	} else {
		fmt.Println("Processes:", processes)
	}
}