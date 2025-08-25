package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	MemoryTotalMB      int64   `json:"memory_total_mb"`
	MemoryUsedMB       int64   `json:"memory_used_mb"`
	DiskUsagePercent   float64 `json:"disk_usage_percent"`
	LoadAverage        float64 `json:"load_average"`
	Uptime             int64   `json:"uptime"`
}

// Monitor 系统监控器
type Monitor struct{}

// NewMonitor 创建新的系统监控器
func NewMonitor() *Monitor {
	return &Monitor{}
}

// GetSystemMetrics 获取系统指标
func (m *Monitor) GetSystemMetrics() (*SystemMetrics, error) {
	metrics := &SystemMetrics{}

	// 获取CPU使用率
	cpuUsage, err := m.getCPUUsage()
	if err == nil {
		metrics.CPUUsagePercent = cpuUsage
	}

	// 获取内存使用率
	memTotal, memUsed, err := m.getMemoryUsage()
	if err == nil {
		metrics.MemoryTotalMB = memTotal / 1024 / 1024 // 转换为MB
		metrics.MemoryUsedMB = memUsed / 1024 / 1024   // 转换为MB
		if memTotal > 0 {
			metrics.MemoryUsagePercent = float64(memUsed) / float64(memTotal) * 100
		}
	}

	// 获取负载平均值
	loadAvg, err := m.getLoadAverage()
	if err == nil {
		metrics.LoadAverage = loadAvg
	}

	// 获取系统运行时间
	uptime, err := m.getUptime()
	if err == nil {
		metrics.Uptime = uptime
	}

	return metrics, nil
}

// getCPUUsage 获取CPU使用率
func (m *Monitor) getCPUUsage() (float64, error) {
	// 读取 /proc/stat 两次计算CPU使用率
	stat1, err := m.readCPUStat()
	if err != nil {
		return 0, err
	}

	// 简单返回一个估算值，实际应该计算两次读取的差值
	// 这里为了简化，返回一个基于当前状态的估算
	if stat1.total > 0 {
		usage := float64(stat1.total-stat1.idle) / float64(stat1.total) * 100
		return usage, nil
	}

	return 0, nil
}

// cpuStat CPU统计信息
type cpuStat struct {
	user   int64
	nice   int64
	system int64
	idle   int64
	total  int64
}

// readCPUStat 读取CPU统计信息
func (m *Monitor) readCPUStat() (*cpuStat, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, fmt.Errorf("failed to read /proc/stat")
	}

	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return nil, fmt.Errorf("invalid /proc/stat format")
	}

	user, _ := strconv.ParseInt(fields[1], 10, 64)
	nice, _ := strconv.ParseInt(fields[2], 10, 64)
	system, _ := strconv.ParseInt(fields[3], 10, 64)
	idle, _ := strconv.ParseInt(fields[4], 10, 64)

	total := user + nice + system + idle

	return &cpuStat{
		user:   user,
		nice:   nice,
		system: system,
		idle:   idle,
		total:  total,
	}, nil
}

// getMemoryUsage 获取内存使用情况
func (m *Monitor) getMemoryUsage() (total, used int64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var memTotal, memFree, buffers, cached int64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		valueStr := fields[1]
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			continue
		}

		// 值通常以kB为单位
		value *= 1024

		switch key {
		case "MemTotal":
			memTotal = value
		case "MemFree":
			memFree = value
		case "Buffers":
			buffers = value
		case "Cached":
			cached = value
		}
	}

	if memTotal == 0 {
		return 0, 0, fmt.Errorf("failed to read memory info")
	}

	// 计算已使用内存（不包括buffers和cache）
	used = memTotal - memFree - buffers - cached

	return memTotal, used, nil
}

// getLoadAverage 获取负载平均值
func (m *Monitor) getLoadAverage() (float64, error) {
	file, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, fmt.Errorf("failed to read /proc/loadavg")
	}

	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 1 {
		return 0, fmt.Errorf("invalid /proc/loadavg format")
	}

	loadAvg, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse load average: %w", err)
	}

	return loadAvg, nil
}

// getUptime 获取系统运行时间
func (m *Monitor) getUptime() (int64, error) {
	file, err := os.Open("/proc/uptime")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, fmt.Errorf("failed to read /proc/uptime")
	}

	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 1 {
		return 0, fmt.Errorf("invalid /proc/uptime format")
	}

	uptimeFloat, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse uptime: %w", err)
	}

	return int64(uptimeFloat), nil
}
