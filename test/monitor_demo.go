package main

import (
	"fmt"
	"log"

	"phoenix-node-agent/internal/gpu"
	"phoenix-node-agent/internal/system"
)

func main() {
	fmt.Println("Phoenix Node Agent - GPU Monitor Test")
	fmt.Println("=====================================")

	// 测试GPU监控
	fmt.Println("\n1. Testing GPU Monitor...")
	gpuMonitor, err := gpu.NewMonitor()
	if err != nil {
		log.Printf("Failed to create GPU monitor: %v", err)
	} else {
		defer gpuMonitor.Close()

		count, err := gpuMonitor.GetGPUCount()
		if err != nil {
			log.Printf("Failed to get GPU count: %v", err)
		} else {
			fmt.Printf("Detected %d GPU(s)\n", count)
		}

		if err := gpuMonitor.RefreshGPUInfo(); err != nil {
			log.Printf("Failed to refresh GPU info: %v", err)
		} else {
			gpus := gpuMonitor.GetGPUInfo()
			for _, gpu := range gpus {
				fmt.Printf("GPU %d: %s, Memory: %dMB/%dMB, Temp: %d°C, Usage: %.1f%%\n",
					gpu.ID, gpu.Name, gpu.MemoryUsedMB, gpu.MemoryTotalMB, gpu.TemperatureC, gpu.UsagePercent)
			}
		}
	}

	// 测试系统监控
	fmt.Println("\n2. Testing System Monitor...")
	sysMonitor := system.NewMonitor()
	metrics, err := sysMonitor.GetSystemMetrics()
	if err != nil {
		log.Printf("Failed to get system metrics: %v", err)
	} else {
		fmt.Printf("CPU Usage: %.1f%%\n", metrics.CPUUsagePercent)
		fmt.Printf("Memory Usage: %.1f%% (%dMB/%dMB)\n", 
			metrics.MemoryUsagePercent, metrics.MemoryUsedMB, metrics.MemoryTotalMB)
		fmt.Printf("Load Average: %.2f\n", metrics.LoadAverage)
		fmt.Printf("Uptime: %d seconds\n", metrics.Uptime)
	}

	fmt.Println("\nTest completed!")
}
