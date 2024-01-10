package mock

import (
	"fmt"
	dto "github.com/prometheus/client_model/go"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

func (p *MockProvider) generateMockMetrics(metricsMap map[string][]*dto.Metric, resourceType string, label []*dto.LabelPair) map[string][]*dto.Metric {
	var (
		cpuMetricSuffix    = "_cpu_usage_seconds_total" // the rate of change of this metric is the cpu usage
		memoryMetricSuffix = "_memory_working_set_bytes"
		cpuDummyValue      = 0.1 // use the time since it's a monotonically increasing value
		memoryDummyValue   = 0.1
	)

	userTime, systemTime, totalCPUTime, err := getUserSystemTotalCPUTime()
	if err != nil {
		fmt.Println("Error getting user, system, and total CPU time:", err)
	} else {
		cpuDummyValue = userTime + systemTime
		fmt.Printf("CPU Usage (User): %.2f\n", userTime)
		fmt.Printf("CPU Usage (System): %.2f\n", systemTime)
		fmt.Printf("CPU Usage (Total): %.2f\n", totalCPUTime)
	}
		
	usedMemory, err := getUsedMemory()
	if err != nil {
		fmt.Println("Error getting used memory:", err)
	} else {
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


func getUserSystemTotalCPUTime() (float64, float64, float64, error) {
	cpuTimes, err := cpu.Times(false)
	if err != nil {
		return 0, 0, 0, err
	}

	var userTime, systemTime, totalCPUTime float64
	for _, ct := range cpuTimes {
		userTime += ct.User
		systemTime += ct.System
		totalCPUTime += ct.Total()
		fmt.Printf("CPU Time (User): %.2f\n", ct.User)
	}

	return userTime, systemTime, totalCPUTime, nil
}


func getUsedMemory() (uint64, error) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return memInfo.Used, nil
}