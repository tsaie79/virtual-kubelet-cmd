package mock

import (
	"fmt"
	dto "github.com/prometheus/client_model/go"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	"syscall"
	v1 "k8s.io/api/core/v1"
	"os"
	"path"
	"strings"
	"bufio"
	"strconv"
	"context"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func (p *MockProvider) generateNodeMetrics(metricsMap map[string][]*dto.Metric, resourceType string, label []*dto.LabelPair) map[string][]*dto.Metric {
	var (
		cpuMetricSuffix    = "_cpu_usage_seconds_total" // the rate of change of this metric is the cpu usage
		memoryMetricSuffix = "_memory_working_set_bytes"
		cpuDummyValue      = 0.0 // use the time since it's a monotonically increasing value
		memoryDummyValue   = 0.0
	)

	userTime, systemTime, _, usedMemory, err := getNodeStats()
	if err != nil {
		log.G(context.Background()).Error("Error getting user, system, total CPU time, and used memory:", err)
	} else {
		cpuDummyValue = userTime + systemTime
		memoryDummyValue = float64(usedMemory)
	}


	if metricsMap == nil {
		metricsMap = map[string][]*dto.Metric{}
	}

	finalCpuMetricName := resourceType + cpuMetricSuffix
	finalMemoryMetricName := resourceType + memoryMetricSuffix

	newCPUMetric := dto.Metric{
		Label: label,
		Counter: &dto.Counter{
			Value: &cpuDummyValue,
		},
	}
	newMemoryMetric := dto.Metric{
		Label: label,
		Gauge: &dto.Gauge{
			Value: &memoryDummyValue,
		},
	}
	// if metric family exists add to metric array
	if cpuMetrics, ok := metricsMap[finalCpuMetricName]; ok {
		metricsMap[finalCpuMetricName] = append(cpuMetrics, &newCPUMetric)
	} else {
		metricsMap[finalCpuMetricName] = []*dto.Metric{&newCPUMetric}
	}
	if memoryMetrics, ok := metricsMap[finalMemoryMetricName]; ok {
		metricsMap[finalMemoryMetricName] = append(memoryMetrics, &newMemoryMetric)
	} else {
		metricsMap[finalMemoryMetricName] = []*dto.Metric{&newMemoryMetric}
	}

	return metricsMap
}

func (p *MockProvider) generatePodMetrics(pod *v1.Pod, metricsMap map[string][]*dto.Metric, resourceType string, label []*dto.LabelPair) (map[string][]*dto.Metric, map[string]int) {
	var (
		cpuMetricSuffix    = "_cpu_usage_seconds_total" // the rate of change of this metric is the cpu usage
		memoryMetricSuffix = "_memory_working_set_bytes"
		cpuDummyValue      = 0.0 // use the time since it's a monotonically increasing value
		memoryDummyValue   = 0.0
	)

	pgids, pgidMap, err := getPgidsFromPod(pod)
	if err != nil {
		log.G(context.Background()).Error("Error getting pgids:", err)
		return nil, nil
	}

	// update label by adding the pgidMap
	pgidMapStr := fmt.Sprintf("%v", pgidMap)
	var pgidLabelKey = "pgidMap"
	label = append(label, &dto.LabelPair{
		Name:  &pgidLabelKey,
		Value: &pgidMapStr,
	})

	var (
		podUserTime = 0.0
		podSystemTime = 0.0
		podRSS = 0.0
		podVMS = 0.0
	)
	for _, pgid := range pgids {
		// log.G(context.Background()).Infof("Pod name: %v, pgid: %v\n", pod.Name, pgid)
		userTime, systemTime, rss, vms, err := getProcessesMetrics(pgid)
		if err != nil {
			log.G(context.Background()).WithField("pgid", pgid).Error("Error getting user, system CPU time, and memory usage:", err)
			continue
		}
		podUserTime += userTime
		podSystemTime += systemTime
		podRSS += rss
		podVMS += vms
	}



	cpuDummyValue = podUserTime + podSystemTime
	memoryDummyValue = podRSS
	log.G(context.Background()).WithField("pod", pod.Name).Infof("Pod CPU time: %.2f, Memory usage: %.2f bytes, %.2f MB\n", cpuDummyValue, memoryDummyValue, memoryDummyValue/1024/1024)

	if metricsMap == nil {
		metricsMap = map[string][]*dto.Metric{}
	}

	// The rest of the function would be similar to generateNodeMetrics, but using cpuDummyValue and memoryDummyValue
	// to populate the metrics for the process group.

	finalCpuMetricName := resourceType + cpuMetricSuffix
	finalMemoryMetricName := resourceType + memoryMetricSuffix

	newCPUMetric := dto.Metric{
		Label: label,
		Counter: &dto.Counter{
			Value: &cpuDummyValue,
		},
	}
	newMemoryMetric := dto.Metric{
		Label: label,
		Gauge: &dto.Gauge{
			Value: &memoryDummyValue,
		},
	}
	// if metric family exists add to metric array
	if cpuMetrics, ok := metricsMap[finalCpuMetricName]; ok {
		metricsMap[finalCpuMetricName] = append(cpuMetrics, &newCPUMetric)
	} else {
		metricsMap[finalCpuMetricName] = []*dto.Metric{&newCPUMetric}
	}
	if memoryMetrics, ok := metricsMap[finalMemoryMetricName]; ok {
		metricsMap[finalMemoryMetricName] = append(memoryMetrics, &newMemoryMetric)
	} else {
		metricsMap[finalMemoryMetricName] = []*dto.Metric{&newMemoryMetric}
	}
	return metricsMap, pgidMap
}


func (p *MockProvider) generateContainerMetrics(c *v1.Container, metricsMap map[string][]*dto.Metric, resourceType string, label []*dto.LabelPair) map[string][]*dto.Metric {
	var (
		cpuMetricSuffix    = "_cpu_usage_seconds_total" // the rate of change of this metric is the cpu usage
		memoryMetricSuffix = "_memory_working_set_bytes"
		cpuDummyValue      = 0. // use the time since it's a monotonically increasing value
		memoryDummyValue   = 0.
	)
	pgid, err := getPgidFromContainer(c)
	if err != nil {
		log.G(context.Background()).Error("Error getting pgid:", err)
		return nil
	}	

	userTime, systemTime, rss, _, err := getProcessesMetrics(pgid)
	if err != nil {
		log.G(context.Background()).WithField("pgid", pgid).Error("Error getting user, system CPU time, and memory usage:", err)
		return nil
	}
	cpuDummyValue = userTime + systemTime
	memoryDummyValue = rss
	log.G(context.Background()).WithField("container", c.Name).Infof("Container CPU time: %.2f, Memory usage: %.2f bytes, %.2f MB\n", cpuDummyValue, memoryDummyValue, memoryDummyValue/1024/1024)

	if metricsMap == nil {
		metricsMap = map[string][]*dto.Metric{}
	}

	// The rest of the function would be similar to generateNodeMetrics, but using cpuDummyValue and memoryDummyValue
	// to populate the metrics for the process group.

	finalCpuMetricName := resourceType + cpuMetricSuffix
	finalMemoryMetricName := resourceType + memoryMetricSuffix

	newCPUMetric := dto.Metric{
		Label: label,
		Counter: &dto.Counter{
			Value: &cpuDummyValue,
		},
	}
	newMemoryMetric := dto.Metric{
		Label: label,
		Gauge: &dto.Gauge{
			Value: &memoryDummyValue,
		},
	}
	
	// if metric family exists add to metric array
	if cpuMetrics, ok := metricsMap[finalCpuMetricName]; ok {
		metricsMap[finalCpuMetricName] = append(cpuMetrics, &newCPUMetric)
	} else {
		log.G(context.Background()).Errorf("cpuMetrics not found: %v\n", finalCpuMetricName)
		metricsMap[finalCpuMetricName] = []*dto.Metric{&newCPUMetric}
	}
	if memoryMetrics, ok := metricsMap[finalMemoryMetricName]; ok {
		metricsMap[finalMemoryMetricName] = append(memoryMetrics, &newMemoryMetric)
	} else {
		log.G(context.Background()).Errorf("memoryMetrics not found: %v\n", finalMemoryMetricName)
		metricsMap[finalMemoryMetricName] = []*dto.Metric{&newMemoryMetric}
	}

	return metricsMap
}


func getNodeStats() (float64, float64, float64, uint64, error) {
    cpuTimes, err := cpu.Times(false)
    if err != nil {
        return 0, 0, 0, 0, err
    }

    var userTime, systemTime, totalCPUTime float64
    for _, ct := range cpuTimes {
        userTime += ct.User
        systemTime += ct.System
        totalCPUTime += ct.Total()
    }

    memInfo, err := mem.VirtualMemory()
    if err != nil {
        return 0, 0, 0, 0, err
    }

    return userTime, systemTime, totalCPUTime, memInfo.Used, nil
}


func getProcessesMetrics(pgid int) (float64, float64, float64, float64, error) {
    pids, err := process.Pids()
    if err != nil {
        return 0, 0, 0, 0, err
    }

    var totalUserTime, totalSystemTime, totalRSS, totalVMS float64

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
			// //print the cmd of the process
			// cmd, err := p.Cmdline()
			// if err == nil {
			// 	log.G(context.Background()).WithField("cmd", cmd).Errorf("Error getting cmd:", err)
			// }

            cpuTimes, err := p.Times()
            if err == nil {
                totalUserTime += cpuTimes.User
                totalSystemTime += cpuTimes.System
            }

            memInfo, err := p.MemoryInfo()
            if err == nil {
                totalRSS += float64(memInfo.RSS)
                totalVMS += float64(memInfo.VMS)
            }
        }
    }

    return totalUserTime, totalSystemTime, totalRSS, totalVMS, nil
}


func getPgidFromContainer(c *v1.Container) (int, error) {
	var pgid_volmount string
	for _, volMount := range c.VolumeMounts {
		if strings.Contains(volMount.Name, "pgid") {
			pgid_volmount = volMount.MountPath
			pgid_volmount = strings.ReplaceAll(pgid_volmount, "~", os.Getenv("HOME"))
			pgid_volmount = strings.ReplaceAll(pgid_volmount, "$HOME", os.Getenv("HOME"))
			break
		}
	}

	if pgid_volmount == "" {
		return 0, fmt.Errorf("pgid volume mount not found")
	}

	pgidFile := path.Join(pgid_volmount, fmt.Sprintf("%s.pgid", c.Name))
	log.G(context.Background()).WithField("container", c.Name).Infof("pgid file: %v\n", pgidFile)
	// open pgid file and read the number in it
	file, err := os.Open(pgidFile)
	if err != nil {
		log.G(context.Background()).WithField("container", c.Name).Errorf("error opening pgid file: %v\n", err)
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	pgidStr := scanner.Text()
	pgidInt, err := strconv.Atoi(pgidStr)
	log.G(context.Background()).WithField("container", c.Name).Infof("pgid: %v\n", pgidInt)
	if err != nil {
		log.G(context.Background()).WithField("container", c.Name).Errorf("error converting pgid to int: %v\n", err)
		return 0, err
	}
	return pgidInt, nil
}

func getPgidsFromPod(pod *v1.Pod) ([]int, map[string]int, error) {
	var pgids []int
	// define a map that stores the container name and its pgid
	var pgidMap = make(map[string]int)
	for _, c := range pod.Spec.Containers {
		var pgid_volmount string
		for _, volMount := range c.VolumeMounts {
			if strings.Contains(volMount.Name, "pgid") {
				pgid_volmount = volMount.MountPath
				pgid_volmount = strings.ReplaceAll(pgid_volmount, "~", os.Getenv("HOME"))
				pgid_volmount = strings.ReplaceAll(pgid_volmount, "$HOME", os.Getenv("HOME"))
				break
			}
		}
	
		if pgid_volmount == "" {
			err := fmt.Errorf("pgid volume mount not found")
			log.G(context.Background()).WithField("container", c.Name).Errorf("pgid volume mount not found: %v\n", err)
			return nil, nil, err
		}

		pgidFile := path.Join(pgid_volmount, fmt.Sprintf("%s.pgid", c.Name))
		// open pgid file and read the number in it
		file, err := os.Open(pgidFile)
		if err != nil {
			log.G(context.Background()).WithField("container", c.Name).Errorf("error opening pgid file: %v\n", err)
			return nil, nil, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Scan()
		pgidStr := scanner.Text()
		pgidInt, err := strconv.Atoi(pgidStr)
		if err != nil {
			log.G(context.Background()).WithField("container", c.Name).Errorf("error converting pgid to int: %v\n", err)
			return nil, nil, err
		}
		pgids = append(pgids, pgidInt)
		pgidMap[c.Name] = pgidInt

		// remove duplicate pgids and preserve only the unique ones
		// https://www.geeksforgeeks.org/how-to-remove-duplicate-values-from-slice-in-golang/
		keys := make(map[int]bool)
		list := []int{}
		for _, entry := range pgids {
			if _, value := keys[entry]; !value {
				keys[entry] = true
				list = append(list, entry)
			}
		}
		pgids = list
	}
	return pgids, pgidMap, nil
}



// get the shell process status from the pgid
func createContainerStatusFromProcessStatus(c *v1.Container, startTime time.Time) *v1.ContainerStatus {
	pgid, err := getPgidFromContainer(c)
	if err != nil {
		log.G(context.Background()).WithField("container", c.Name).Errorf("Error getting pgid:", err)
		return nil
	}

	pids, err := process.Pids()
	if err != nil {
		log.G(context.Background()).WithField("container", c.Name).Error("Error getting pids:", err)
	}

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
			//print the cmd of the process
			cmd, _ := p.Cmdline()
			// if err == nil {
			//  log.G(context.Background()).WithField("cmd", cmd).Errorf("Error getting cmd:", err)
			// }

			// get the process status
			status, err := p.Status()
			if err != nil {
				log.G(context.Background()).WithField("container", c.Name).Errorf("Error getting process status:", err)
				return nil
			}
			processStatus = append(processStatus, status)
			log.G(context.Background()).WithField("cmd", cmd).Infof("Process status: %v\n", status)
		}
	}


	var containerStatus *v1.ContainerStatus
	var containerState *v1.ContainerState
	// if one of the process status is "R" (running), then the container is running
	for _, status := range processStatus {
		if status == "R" {
			containerState = &v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: metav1.NewTime(startTime),
				},
			}
			containerStatus = &v1.ContainerStatus{
				Name:         c.Name,
				State:        *containerState,
				Ready:        true,
				RestartCount: 0,
				Image:        c.Image,
				ImageID:      "",
				ContainerID:  "",
			}
			return containerStatus
		}
	}

	// if process status has only "zombie" processes, then the container is Completed
	containerState = &v1.ContainerState{
		Terminated: &v1.ContainerStateTerminated{
			Reason:      "",
			Message:     "status: " + strings.Join(processStatus, ", "),
			StartedAt:   metav1.NewTime(startTime),
			FinishedAt:  metav1.NewTime(time.Now()),
			ContainerID: "",
		},
	}
	containerStatus = &v1.ContainerStatus{
		Name:         c.Name,
		State:        *containerState,
		Ready:        true,
		RestartCount: 0,
		Image:        c.Image,
		ImageID:      "",
		ContainerID:  "",
	}
	return containerStatus
}

// create the pod spec status from the container status
func createPodSpecStatusFromContainerStatus(pod *v1.Pod, startTime time.Time) *v1.Pod {
	var containerStatuses []v1.ContainerStatus
	for _, c := range pod.Spec.Containers {
		containerStatus := createContainerStatusFromProcessStatus(&c)
		containerStatuses = append(containerStatuses, *containerStatus)
	}

	// update the pod status
	pod.Status = v1.PodStatus{
		Phase:             v1.PodRunning,
		ContainerStatuses: containerStatuses,
	}

	// if the messages in the container state are all "Zombie", then the pod is completed
	checkProcessStatusIsZombie := func(pod *v1.Pod) bool {
		counter := 0
		for _, c := range pod.Status.ContainerStatuses {
			if c.State.Terminated != nil {
				if c.State.Terminated.Message == "status: Z" {
					counter++
				}
			}
		}
		return counter == len(pod.Status.ContainerStatuses)
	}

	if checkProcessStatusIsZombie(pod) {
		pod.Status.Phase = v1.PodSucceeded
	}
	return pod
}