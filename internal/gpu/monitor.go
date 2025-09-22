package gpu

import (
	"fmt"
	"sync"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// GPUInfo GPU信息
type GPUInfo struct {
	ID            int     `json:"id"`
	TemperatureC  int     `json:"temperature_c"`
	MemoryTotalMB int     `json:"memory_total_mb"`
	MemoryUsedMB  int     `json:"memory_used_mb"`
	Name          string  `json:"name"`
	UUID          string  `json:"uuid"`
	Busy          bool    `json:"busy"`
	UsagePercent  float64 `json:"usage_percent"`
}

// Monitor GPU监控器
type Monitor struct {
	mu   sync.RWMutex
	gpus []GPUInfo
}

// NewMonitor 创建新的GPU监控器
func NewMonitor() (*Monitor, error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}

	return &Monitor{}, nil
}

// Close 关闭监控器
func (m *Monitor) Close() error {
	ret := nvml.Shutdown()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
	}
	return nil
}

// GetGPUCount 获取GPU数量
func (m *Monitor) GetGPUCount() (int, error) {
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}
	return count, nil
}

// RefreshGPUInfo 刷新GPU信息
func (m *Monitor) RefreshGPUInfo() error {
	count, err := m.GetGPUCount()
	if err != nil {
		return err
	}

	gpus := make([]GPUInfo, count)

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("failed to get device handle for GPU %d: %v", i, nvml.ErrorString(ret))
		}

		// 获取GPU名称
		name, ret := device.GetName()
		if ret != nvml.SUCCESS {
			name = "Unknown"
		}

		// 获取GPU UUID
		uuid, ret := device.GetUUID()
		if ret != nvml.SUCCESS {
			uuid = "Unknown"
		}

		// 获取温度
		temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
		if ret != nvml.SUCCESS {
			temp = 0
		}

		// 获取内存信息
		memInfo, ret := device.GetMemoryInfo()
		totalMB := int(memInfo.Total / 1024 / 1024)
		usedMB := int(memInfo.Used / 1024 / 1024)
		if ret != nvml.SUCCESS {
			totalMB = 0
			usedMB = 0
		}

		// 获取利用率
		utilization, ret := device.GetUtilizationRates()
		var usagePercent float64
		if ret == nvml.SUCCESS {
			usagePercent = float64(utilization.Gpu)
		}

		// 判断GPU是否忙碌（基于内存使用率和利用率）
		busy := false
		if totalMB > 0 {
			memUsagePercent := float64(usedMB) / float64(totalMB) * 100
			busy = memUsagePercent > 10.0 || usagePercent > 10.0
		}

		gpus[i] = GPUInfo{
			ID:            i,
			TemperatureC:  int(temp),
			MemoryTotalMB: totalMB,
			MemoryUsedMB:  usedMB,
			Name:          name,
			UUID:          uuid,
			Busy:          busy,
			UsagePercent:  usagePercent,
		}
	}

	m.mu.Lock()
	m.gpus = gpus
	m.mu.Unlock()

	return nil
}

// GetGPUInfo 获取所有GPU信息
func (m *Monitor) GetGPUInfo() []GPUInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	result := make([]GPUInfo, len(m.gpus))
	copy(result, m.gpus)
	return result
}

// GetGPUByID 根据ID获取GPU信息
func (m *Monitor) GetGPUByID(id int) (GPUInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if id < 0 || id >= len(m.gpus) {
		return GPUInfo{}, false
	}

	return m.gpus[id], true
}

// IsGPUAvailable 检查GPU是否可用（未被占用）
func (m *Monitor) IsGPUAvailable(id int) bool {
	gpu, exists := m.GetGPUByID(id)
	if !exists {
		return false
	}
	return !gpu.Busy
}

// IsGPUInUse 检查GPU是否正在使用
func (m *Monitor) IsGPUInUse(id int) bool {
	gpu, exists := m.GetGPUByID(id)
	if !exists {
		return false
	}
	return gpu.Busy
}

// GetAvailableGPUs 获取所有可用的GPU ID列表
func (m *Monitor) GetAvailableGPUs() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var available []int
	for _, gpu := range m.gpus {
		if !gpu.Busy {
			available = append(available, gpu.ID)
		}
	}
	return available
}
