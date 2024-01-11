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
		fmt.Println("Error getting user, system, total CPU time, and used memory:", err)
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

	pgids, pgid_map, err := getPgidsFromPod(pod)
	if err != nil {
		fmt.Println("Error getting pgids:", err)
		return nil, nil
	}

	var (
		podUserTime = 0.0
		podSystemTime = 0.0
		podRSS = 0.0
		podVMS = 0.0
	)
	for _, pgid := range pgids {
		fmt.Printf("readout pgid (p): %v\n", pgid)
		userTime, systemTime, rss, vms, err := getProcessesMetrics(pgid)
		if err != nil {
			fmt.Println("Error getting user, system CPU time, and memory usage:", err)
			continue
		}
		podUserTime += userTime
		podSystemTime += systemTime
		podRSS += rss
		podVMS += vms
	}


	if err != nil {
		fmt.Println("Error getting user, system CPU time, and memory usage:", err)
	} else {
		cpuDummyValue = podUserTime + podSystemTime
		memoryDummyValue = podRSS
		fmt.Printf("Pod CPU time: %.2f\n", cpuDummyValue)
		fmt.Printf("Pod Memory usage: %.2f bytes, %.2f MB\n", memoryDummyValue, memoryDummyValue/1024/1024)
	}

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

	return metricsMap, pgid_map
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
		fmt.Println("Error getting pgid:", err)
		return nil
	}	

	userTime, systemTime, rss, _, err := getProcessesMetrics(pgid)
	if err != nil {
		fmt.Println("Error getting user, system CPU time, and memory usage:", err)
		return nil
	}
	cpuDummyValue = userTime + systemTime
	memoryDummyValue = rss

	fmt.Printf("Container CPU time: %.2f\n", cpuDummyValue)
	fmt.Printf("Container Memory usage: %.2f bytes, %.2f MB\n", memoryDummyValue, memoryDummyValue/1024/1024)

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
		fmt.Printf("cpuMetrics not found: %v\n", finalCpuMetricName)
		metricsMap[finalCpuMetricName] = []*dto.Metric{&newCPUMetric}
	}
	if memoryMetrics, ok := metricsMap[finalMemoryMetricName]; ok {
		metricsMap[finalMemoryMetricName] = append(memoryMetrics, &newMemoryMetric)
	} else {
		fmt.Printf("memoryMetrics not found: %v\n", finalMemoryMetricName)
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
			//print the cmd of the process
			cmd, err := p.Cmdline()
			if err == nil {
				fmt.Printf("pid: %v, pgid: %v, cmd: %v\n", pid, pgid, cmd)
			}

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

	pgid_file := path.Join(pgid_volmount, fmt.Sprintf("%s.pgid", c.Name))
	fmt.Printf("pgid file (c): %v\n", pgid_file)
	// open pgid file and read the number in it
	file, err := os.Open(pgid_file)
	if err != nil {
		return 0, fmt.Errorf("error opening pgid file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	pgid_str := scanner.Text()
	pgid_int, err := strconv.Atoi(pgid_str)
	fmt.Printf("readout pgid: %v\n", pgid_int)
	if err != nil {
		return 0, fmt.Errorf("error converting pgid to int: %v", err)
	}
	return pgid_int, nil
}

func getPgidsFromPod(pod *v1.Pod) ([]int, map[string]int, error) {
	var pgids []int
	// define a map that stores the container name and its pgid
	var pgid_map = make(map[string]int)
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
			return nil, nil, fmt.Errorf("pgid volume mount not found")
		}

		pgid_file := path.Join(pgid_volmount, fmt.Sprintf("%s.pgid", c.Name))
		// open pgid file and read the number in it
		file, err := os.Open(pgid_file)
		if err != nil {
			return nil, nil, fmt.Errorf("error opening pgid file: %v", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Scan()
		pgid_str := scanner.Text()
		pgid_int, err := strconv.Atoi(pgid_str)
		if err != nil {
			return nil, nil, fmt.Errorf("error converting pgid to int: %v", err)
		}
		pgids = append(pgids, pgid_int)
		pgid_map[c.Name] = pgid_int

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
	return pgids, pgid_map, nil
}

