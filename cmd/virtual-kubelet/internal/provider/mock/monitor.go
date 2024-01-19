package mock

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *MockProvider) generateNodeMetrics(metricsMap map[string][]*dto.Metric, resourceType string, label []*dto.LabelPair) map[string][]*dto.Metric {
	const (
		cpuMetricSuffix    = "_cpu_usage_seconds_total" // the rate of change of this metric is the cpu usage
		memoryMetricSuffix = "_memory_working_set_bytes"
	)

	// Initialize CPU and memory values
	cpuValue, memoryValue := 0.0, 0.0

	// Get node stats
	userTime, systemTime, _, usedMemory, err := getNodeStats()
	if err != nil {
		log.G(context.Background()).Error("Error getting user, system, total CPU time, and used memory:", err)
	} else {
		// Update CPU and memory values
		cpuValue = userTime + systemTime
		memoryValue = float64(usedMemory)
	}

	// Initialize metrics map if nil
	if metricsMap == nil {
		metricsMap = map[string][]*dto.Metric{}
	}

	// Generate metric names
	finalCpuMetricName := resourceType + cpuMetricSuffix
	finalMemoryMetricName := resourceType + memoryMetricSuffix

	// Create new CPU and memory metrics
	newCPUMetric := &dto.Metric{
		Label: label,
		Counter: &dto.Counter{
			Value: &cpuValue,
		},
	}
	newMemoryMetric := &dto.Metric{
		Label: label,
		Gauge: &dto.Gauge{
			Value: &memoryValue,
		},
	}

	// Add new metrics to metrics map
	metricsMap = addMetricToMap(metricsMap, finalCpuMetricName, newCPUMetric)
	metricsMap = addMetricToMap(metricsMap, finalMemoryMetricName, newMemoryMetric)

	return metricsMap
}

func (p *MockProvider) generatePodMetrics(pod *v1.Pod, metricsMap map[string][]*dto.Metric, resourceType string, label []*dto.LabelPair) (map[string][]*dto.Metric, map[string]int) {
	const (
		cpuMetricSuffix    = "_cpu_usage_seconds_total" // the rate of change of this metric is the cpu usage
		memoryMetricSuffix = "_memory_working_set_bytes"
	)

	// Initialize CPU and memory values
	cpuValue, memoryValue := 0.0, 0.0

	// Get process group IDs from pod
	pgids, pgidMap, err := getPgidsFromPod(pod)
	if err != nil {
		log.G(context.Background()).Error("Error getting pgids:", err)
		return nil, nil
	}

	// Update label by adding the pgidMap
	pgidMapStr := fmt.Sprintf("%v", pgidMap)
	pgidLabelKey := "pgidMap"
	label = append(label, &dto.LabelPair{
		Name:  &pgidLabelKey,
		Value: &pgidMapStr,
	})

	// Get process metrics for each process group ID
	for _, pgid := range pgids {
		userTime, systemTime, rss, _, err := getProcessesMetrics(pgid)
		if err != nil {
			log.G(context.Background()).WithField("pgid", pgid).Error("Error getting user, system CPU time, and memory usage:", err)
			continue
		}

		// Update CPU and memory values
		cpuValue += userTime + systemTime
		memoryValue += rss
	}

	log.G(context.Background()).WithField("pod", pod.Name).Infof("Pod CPU time: %.2f, Memory usage: %.2f bytes, %.2f MB\n", cpuValue, memoryValue, memoryValue/1024/1024)

	// Initialize metrics map if nil
	if metricsMap == nil {
		metricsMap = map[string][]*dto.Metric{}
	}

	// Generate metric names
	finalCpuMetricName := resourceType + cpuMetricSuffix
	finalMemoryMetricName := resourceType + memoryMetricSuffix

	// Create new CPU and memory metrics
	newCPUMetric := &dto.Metric{
		Label: label,
		Counter: &dto.Counter{
			Value: &cpuValue,
		},
	}
	newMemoryMetric := &dto.Metric{
		Label: label,
		Gauge: &dto.Gauge{
			Value: &memoryValue,
		},
	}

	// Add new metrics to metrics map
	metricsMap = addMetricToMap(metricsMap, finalCpuMetricName, newCPUMetric)
	metricsMap = addMetricToMap(metricsMap, finalMemoryMetricName, newMemoryMetric)

	return metricsMap, pgidMap
}

func (p *MockProvider) generateContainerMetrics(c *v1.Container, metricsMap map[string][]*dto.Metric, resourceType string, label []*dto.LabelPair, pgidFile string) map[string][]*dto.Metric {
	const (
		cpuMetricSuffix    = "_cpu_usage_seconds_total" // the rate of change of this metric is the cpu usage
		memoryMetricSuffix = "_memory_working_set_bytes"
	)

	// Initialize CPU and memory values
	cpuValue, memoryValue := 0.0, 0.0

	// Get process group ID from container
	pgid, err := getPgidFromPgidFile(pgidFile)
	if err != nil {
		log.G(context.Background()).Error("Error getting pgid:", err)
		return nil
	}

	// Get process metrics
	userTime, systemTime, rss, _, err := getProcessesMetrics(pgid)
	if err != nil {
		log.G(context.Background()).WithField("pgid", pgid).Error("Error getting user, system CPU time, and memory usage:", err)
		return nil
	}

	// Update CPU and memory values
	cpuValue = userTime + systemTime
	memoryValue = rss

	log.G(context.Background()).WithField("container", c.Name).Infof("Container CPU time: %.2f, Memory usage: %.2f bytes, %.2f MB\n", cpuValue, memoryValue, memoryValue/1024/1024)

	// Initialize metrics map if nil
	if metricsMap == nil {
		metricsMap = map[string][]*dto.Metric{}
	}

	// Generate metric names
	finalCpuMetricName := resourceType + cpuMetricSuffix
	finalMemoryMetricName := resourceType + memoryMetricSuffix

	// Create new CPU and memory metrics
	newCPUMetric := &dto.Metric{
		Label: label,
		Counter: &dto.Counter{
			Value: &cpuValue,
		},
	}
	newMemoryMetric := &dto.Metric{
		Label: label,
		Gauge: &dto.Gauge{
			Value: &memoryValue,
		},
	}

	// Add new metrics to metrics map
	metricsMap = addMetricToMap(metricsMap, finalCpuMetricName, newCPUMetric)
	metricsMap = addMetricToMap(metricsMap, finalMemoryMetricName, newMemoryMetric)

	return metricsMap
}

// addMetricToMap adds a new metric to the metrics map.
func addMetricToMap(metricsMap map[string][]*dto.Metric, metricName string, newMetric *dto.Metric) map[string][]*dto.Metric {
	if existingMetrics, ok := metricsMap[metricName]; ok {
		metricsMap[metricName] = append(existingMetrics, newMetric)
	} else {
		log.G(context.Background()).Errorf("Metrics not found: %v\n", metricName)
		metricsMap[metricName] = []*dto.Metric{newMetric}
	}
	return metricsMap
}

// getNodeStats calculates and returns the total user time, total system time, total CPU time, and used memory for the node.
func getNodeStats() (totalUserTime float64, totalSystemTime float64, totalCPUTime float64, usedMemory uint64, err error) {
	// Get the CPU times
	cpuTimes, err := cpu.Times(false)
	if err != nil {
		return
	}

	// Iterate over each CPU time and accumulate the user time, system time, and total CPU time
	for _, ct := range cpuTimes {
		totalUserTime += ct.User
		totalSystemTime += ct.System
		totalCPUTime += ct.Total()
	}

	// Get the virtual memory information
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	// Get the used memory
	usedMemory = memInfo.Used

	return
}

// getProcessesMetrics calculates and returns the total user time, total system time, total RSS, and total VMS for all processes in a process group.
func getProcessesMetrics(pgid int) (totalUserTime float64, totalSystemTime float64, totalRSS float64, totalVMS float64, err error) {
	// Get the list of all process IDs
	pids, err := process.Pids()
	if err != nil {
		return
	}

	// Iterate over each process ID
	for _, pid := range pids {
		// Create a new Process instance
		p, err := process.NewProcess(pid)
		if err != nil {
			continue
		}

		// Get the process group ID of the process
		processPgid, err := syscall.Getpgid(int(pid))
		if err != nil {
			continue
		}

		// If the process is in the target process group, accumulate its metrics
		if processPgid == pgid {
			// Accumulate the CPU times
			if cpuTimes, err := p.Times(); err == nil {
				totalUserTime += cpuTimes.User
				totalSystemTime += cpuTimes.System
			}

			// Accumulate the memory information
			if memInfo, err := p.MemoryInfo(); err == nil {
				totalRSS += float64(memInfo.RSS)
				totalVMS += float64(memInfo.VMS)
			}
		}
	}

	return
}

// getPgidFromPgidFile retrieves the process group ID (pgid) from a file.
func getPgidFromPgidFile(pgidFilePath string) (int, error) {
	// Open the pgid file
	file, err := os.Open(pgidFilePath)
	if err != nil {
		log.G(context.Background()).WithField("pgidFilePath", pgidFilePath).Error("Failed to open pgid file:", err)
		return 0, err
	}
	defer file.Close()

	// Create a new scanner to read the file
	scanner := bufio.NewScanner(file)
	scanner.Scan()

	// Get the pgid as a string
	pgidString := scanner.Text()

	// Convert the pgid string to an integer
	pgid, err := strconv.Atoi(pgidString)
	if err != nil {
		log.G(context.Background()).WithField("pgidFilePath", pgidFilePath).Error("Failed to convert pgid to integer:", err)
		return 0, err
	}

	// Log the pgid
	log.G(context.Background()).WithField("pgidFilePath", pgidFilePath).Infof("pgid: %v\n", pgid)

	return pgid, nil
}

// getPgidsFromPod retrieves the process group IDs (pgids) from a pod.
func getPgidsFromPod(pod *v1.Pod) ([]int, map[string]int, error) {
	var pgids []int
	pgidMap := make(map[string]int)

	// Iterate over each container in the pod
	for _, container := range pod.Spec.Containers {
		// Construct the path to the pgid file
		pgidFilePath := path.Join(os.Getenv("HOME"), ".pgid", fmt.Sprintf("%s_%s_%s.pgid", pod.Namespace, pod.Name, container.Name))

		// Read the pgid from the file
		pgid, err := readPgidFromFile(pgidFilePath)
		if err != nil {
			log.G(context.Background()).WithField("container", container.Name).Error(err)
			return nil, nil, err
		}

		// Add the pgid to the list and map
		pgids = appendUnique(pgids, pgid)
		pgidMap[container.Name] = pgid
	}

	return pgids, pgidMap, nil
}



// createContainerStatusFromProcessStatus creates a container status from process status.
func createContainerStatusFromProcessStatus(c *v1.Container, prevContainerState map[string]string, containerStartTime map[string]metav1.Time, containerFinishTime map[string]metav1.Time, pgidFile string) *v1.ContainerStatus {
	// Get the process group ID (pgid) from the container
	pgid, err := getPgidFromPgidFile(pgidFile)
	if err != nil {
		logError("Error getting pgid:", c.Name, err)
		return nil
	}

	// Get the process IDs (pids)
	pids, err := process.Pids()
	if err != nil {
		logError("Error getting pids:", c.Name, err)
		return nil
	}

	// Get the process status for each pid
	processStatus := getProcessStatus(pids, pgid, c.Name)
	if processStatus == nil {
		return nil
	}

	log.G(context.Background()).WithField("container", c.Name).Infof("Previous state: %v\n", prevContainerState[c.Name])
	// Determine the container status 
	containerStatus := determineContainerStatus(c, processStatus, pgid, containerStartTime[c.Name].Time, containerFinishTime[c.Name].Time, prevContainerState[c.Name])
	return containerStatus
}

// createPodStatusFromContainerStatus creates the pod status from the container status.
func (*MockProvider) createPodStatusFromContainerStatus(pod *v1.Pod, prevContainerStateStrings map[string]string) *v1.Pod {
	var containerStatuses []v1.ContainerStatus
	// Get the previous status of the containers in the pod and previous start time and finish time of the containers in the pod
	containerStartTime := make(map[string]metav1.Time)
	containerFinishTime := make(map[string]metav1.Time)


	for _, containerStatus := range pod.Status.ContainerStatuses {
		log.G(context.Background()).WithField("container", containerStatus.Name).Infof("Container status: %v\n", containerStatus)
		prevContainerStateString := prevContainerStateStrings[containerStatus.Name]
		switch prevContainerStateString {
		case "Running":
			if containerStatus.State.Running != nil {
				containerStartTime[containerStatus.Name] = containerStatus.State.Running.StartedAt
				containerFinishTime[containerStatus.Name] = metav1.NewTime(time.Now())
				log.G(context.Background()).WithField("container", containerStatus.Name).Infof("prev state: %v, current state: %v\n", prevContainerStateString, containerStatus.State.Running)
			}else if containerStatus.State.Terminated != nil {
				containerStartTime[containerStatus.Name] = containerStatus.State.Terminated.StartedAt
				containerFinishTime[containerStatus.Name] = metav1.NewTime(time.Now())
				log.G(context.Background()).WithField("container", containerStatus.Name).Infof("prev state: %v, current state: %v\n", prevContainerStateString, containerStatus.State.Terminated)

			}
		case "Waiting":
			if containerStatus.State.Waiting != nil {
				containerStartTime[containerStatus.Name] = metav1.NewTime(time.Now())
				containerFinishTime[containerStatus.Name] = metav1.NewTime(time.Now())
				log.G(context.Background()).WithField("container", containerStatus.Name).Infof("prev state: %v, current state: %v\n", prevContainerStateString, containerStatus.State.Waiting)

			}else if containerStatus.State.Running != nil {
				containerStartTime[containerStatus.Name] = metav1.NewTime(time.Now())
				containerFinishTime[containerStatus.Name] = metav1.NewTime(time.Now())
				log.G(context.Background()).WithField("container", containerStatus.Name).Infof("prev state: %v, current state: %v\n", prevContainerStateString, containerStatus.State.Running)

			}
		case "Terminated":
			if containerStatus.State.Terminated != nil {
				containerStartTime[containerStatus.Name] = containerStatus.State.Terminated.StartedAt
				containerFinishTime[containerStatus.Name] = containerStatus.State.Terminated.FinishedAt
				log.G(context.Background()).WithField("container", containerStatus.Name).Infof("prev state: %v, current state: %v\n", prevContainerStateString, containerStatus.State.Terminated)

			}
		default:
			containerStartTime[containerStatus.Name] = metav1.NewTime(time.Now())
			containerFinishTime[containerStatus.Name] = metav1.NewTime(time.Now())
			log.G(context.Background()).WithField("container", containerStatus.Name).Infof("prev state: %v, current state: %v\n", prevContainerStateString, containerStatus.State)

		}
	}


	// Create a container status for each container in the pod
	for _, container := range pod.Spec.Containers {
		log.G(context.Background()).WithField("container", container.Name).Infof("prevContainerState: %v\n", prevContainerStateStrings)
		pgidFile := path.Join(os.Getenv("HOME"), ".pgid", fmt.Sprintf("%s_%s_%s.pgid", pod.Namespace, pod.Name, container.Name))
		// log.G(context.Background()).WithField("container", container.Name).Infof("prevcntainerstate: %v\n", prevContainerState[container.Name])
		containerStatus := createContainerStatusFromProcessStatus(&container, prevContainerStateStrings, containerStartTime, containerFinishTime, pgidFile)
		containerStatuses = append(containerStatuses, *containerStatus)
	}
	
	// Update the pod status
	pod.Status = v1.PodStatus{
		Phase:             v1.PodRunning,
		ContainerStatuses: containerStatuses,
	}

	// If all the containers in the pod are in the "Zombie" state, then the pod is completed
	if allContainersAreZombies(pod) {
		pod.Status.Phase = v1.PodSucceeded
	}

	return pod
}

// getPodFinishTime gets the finish time of the pod.
func getPodFinishTime(pod *v1.Pod) time.Time {
	var finishTime time.Time
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil {
			finishTime = containerStatus.State.Terminated.FinishedAt.Time
		}
	}
	return finishTime
}

// allContainersAreZombies checks if all the containers in the pod are in the "Zombie" state.
func allContainersAreZombies(pod *v1.Pod) bool {
	zombieCounter := 0
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason == "Completed" && strings.Contains(containerStatus.State.Terminated.Message, "Remaining processes are zombies") {
			zombieCounter++
		}
	}
	return zombieCounter == len(pod.Status.ContainerStatuses)
}

// logError logs an error message.
func logError(message string, containerName string, err error) {
	log.G(context.Background()).WithField("container", containerName).Errorf(message, err)
}

// getProcessStatus gets the process status for each pid.
func getProcessStatus(pids []int32, pgid int, containerName string) []string {
	var processStatus []string
	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err != nil {
			continue
		}

		processPgid, err := syscall.Getpgid(int(pid))
		if err != nil {
			continue
		}

		if processPgid == pgid {
			cmd, _ := p.Cmdline()
			status, err := p.Status()
			if err != nil {
				logError("Error getting process status:", containerName, err)
				return nil
			}
			processStatus = append(processStatus, status)
			log.G(context.Background()).WithField("cmd", cmd).Infof("Process status: %v\n", status)
		}
	}
	return processStatus
}

// determineContainerStatus determines the container status.
func determineContainerStatus(c *v1.Container, processStatus []string, pgid int, containerStartTime time.Time, containerFinishTime time.Time, prevContainerStateString string) *v1.ContainerStatus {
	var containerStatus *v1.ContainerStatus
	var containerState *v1.ContainerState
	var currentContainerState string

	log.G(context.Background()).WithField("container", c.Name).Infof("Previous state: %v\n", prevContainerStateString)
	var allZ = true
	for _, status := range processStatus {
		if status != "Z" {
			allZ = false
			break
		}
	}

	if allZ {
		currentContainerState = "Terminated"
	}else{
		currentContainerState = "Running"
	}

	log.G(context.Background()).WithField("container", c.Name).Infof("Current state: %v\n", currentContainerState)

	switch prevContainerStateString {
	case "Waiting":
		if currentContainerState == "Running" {
			log.G(context.Background()).WithField("container", c.Name).Infof("Transitioning from Waiting to Running\n")
			containerState = &v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: metav1.NewTime(containerStartTime),
				},
			}
		}
	case "Running":
		if currentContainerState == "Running" {
			log.G(context.Background()).WithField("container", c.Name).Infof("Transitioning from Running to Running\n")
			containerState = &v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: metav1.NewTime(containerStartTime),
				},
			}
		} else if currentContainerState == "Terminated" {
			log.G(context.Background()).WithField("container", c.Name).Infof("Transitioning from Running to Terminated\n")
			containerState = &v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					StartedAt:  metav1.NewTime(containerStartTime),
					FinishedAt: metav1.NewTime(containerFinishTime),
					ExitCode:   0,
					Reason:     "Completed",
					Message:    "Remaining processes are zombies",
				},
			}
		}
	case "Terminated":
		if currentContainerState == "Terminated" {
			log.G(context.Background()).WithField("container", c.Name).Infof("Transitioning from Terminated to Terminated\n")
			containerState = &v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					StartedAt:  metav1.NewTime(containerStartTime),
					FinishedAt: metav1.NewTime(containerFinishTime),
					ExitCode:   0,
					Reason:     "Completed",
					Message:    "Remaining processes are zombies",
				},
			}
		}
	}

	containerStatus = createContainerStatus(c, containerState, pgid)
	return containerStatus
}

// createContainerStatus creates a container status.
func createContainerStatus(c *v1.Container, containerState *v1.ContainerState, pgid int) *v1.ContainerStatus {
	log.G(context.Background()).WithField("container", c.Name).Infof("Container state: %v\n", containerState)
	return &v1.ContainerStatus{
		Name:         c.Name,
		State:        *containerState,
		Ready:        true,
		RestartCount: 0,
		Image:        c.Image,
		ImageID:      "",
		ContainerID:  fmt.Sprintf("%v", pgid),
	}
}

// readPgidFromFile reads a pgid from a file.
func readPgidFromFile(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	pgidString := scanner.Text()

	return strconv.Atoi(pgidString)
}

// appendUnique appends a value to a slice if it's not already in the slice.
func appendUnique(slice []int, value int) []int {
	for _, v := range slice {
		if v == value {
			return slice
		}
	}
	return append(slice, value)
}