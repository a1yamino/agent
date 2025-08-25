package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CreateRequest 容器创建请求
type CreateRequest struct {
	ClaimID      string            `json:"claim_id" binding:"required"`
	Image        string            `json:"image" binding:"required"`
	GPUIDs       []int             `json:"gpu_ids" binding:"required"`
	PortMappings []PortMapping     `json:"port_mappings"`
	EnvVars      []string          `json:"env_vars"`
	Command      []string          `json:"command,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Volumes      map[string]string `json:"volumes,omitempty"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port" binding:"required"`
	ContainerPort int    `json:"container_port" binding:"required"`
	Protocol      string `json:"protocol,omitempty"` // tcp, udp
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	ID      string            `json:"id"`
	ClaimID string            `json:"claim_id"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	GPUIDs  []int             `json:"gpu_ids"`
	Ports   map[string]string `json:"ports"`
	Created int64             `json:"created"`
	Started int64             `json:"started"`
	Labels  map[string]string `json:"labels"`
}

// DockerContainer Docker容器信息结构（用于解析docker inspect输出）
type DockerContainer struct {
	ID      string `json:"Id"`
	Created string `json:"Created"`
	State   struct {
		Status     string `json:"Status"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
	} `json:"State"`
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
		Cmd    []string          `json:"Cmd"`
	} `json:"Config"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

// Manager 容器管理器
type Manager struct {
	mu         sync.RWMutex
	containers map[string]ContainerInfo // containerID -> ContainerInfo
}

// NewManager 创建新的容器管理器
func NewManager() (*Manager, error) {
	// 检查Docker是否可用
	if err := exec.Command("docker", "version").Run(); err != nil {
		return nil, fmt.Errorf("docker is not available: %w", err)
	}

	return &Manager{
		containers: make(map[string]ContainerInfo),
	}, nil
}

// Close 关闭管理器
func (m *Manager) Close() error {
	return nil
}

// CreateContainer 创建并启动容器
func (m *Manager) CreateContainer(ctx context.Context, req *CreateRequest) (string, error) {
	// 构建Docker运行命令
	args := []string{"run", "-d"}

	// 添加GPU设备
	if len(req.GPUIDs) > 0 {
		gpuList := make([]string, len(req.GPUIDs))
		for i, id := range req.GPUIDs {
			gpuList[i] = strconv.Itoa(id)
		}
		args = append(args, "--gpus", fmt.Sprintf("device=%s", strings.Join(gpuList, ",")))
	}

	// 添加端口映射
	for _, pm := range req.PortMappings {
		protocol := pm.Protocol
		if protocol == "" {
			protocol = "tcp"
		}
		portMapping := fmt.Sprintf("%d:%d/%s", pm.HostPort, pm.ContainerPort, protocol)
		args = append(args, "-p", portMapping)
	}

	// 添加环境变量
	for _, env := range req.EnvVars {
		args = append(args, "-e", env)
	}

	// 添加卷挂载
	for hostPath, containerPath := range req.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// 添加标签
	args = append(args,
		"--label", fmt.Sprintf("utopia.claim_id=%s", req.ClaimID),
		"--label", fmt.Sprintf("utopia.gpu_ids=%s", strings.Join(convertIntSliceToStringSlice(req.GPUIDs), ",")),
		"--label", "utopia.managed=true",
		"--label", "utopia.node_type=gpu",
	)

	// 添加容器名称
	containerName := fmt.Sprintf("utopia-claim-%s", req.ClaimID)
	args = append(args, "--name", containerName)

	// 添加重启策略
	args = append(args, "--restart", "unless-stopped")

	// 添加工作目录
	if req.WorkingDir != "" {
		args = append(args, "--workdir", req.WorkingDir)
	}

	// 添加镜像
	args = append(args, req.Image)

	// 添加命令
	if len(req.Command) > 0 {
		args = append(args, req.Command...)
	}

	// 执行Docker命令
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	containerID := strings.TrimSpace(string(output))

	// 获取容器详细信息
	if err := m.RefreshContainer(ctx, containerID); err != nil {
		return "", fmt.Errorf("failed to refresh container info: %w", err)
	}

	return containerID, nil
}

// RemoveContainer 停止并删除容器
func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	// 停止容器
	stopCmd := exec.CommandContext(ctx, "docker", "stop", "-t", "30", containerID)
	if err := stopCmd.Run(); err != nil {
		// 如果停止失败，记录但继续删除
		fmt.Printf("Warning: failed to stop container %s: %v\n", containerID, err)
	}

	// 删除容器
	removeCmd := exec.CommandContext(ctx, "docker", "rm", "-f", "-v", containerID)
	if err := removeCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// 从本地缓存中移除
	m.mu.Lock()
	delete(m.containers, containerID)
	m.mu.Unlock()

	return nil
}

// GetContainer 获取容器信息
func (m *Manager) GetContainer(containerID string) (ContainerInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.containers[containerID]
	return info, exists
}

// ListContainers 列出所有容器
func (m *Manager) ListContainers() []ContainerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var containers []ContainerInfo
	for _, info := range m.containers {
		containers = append(containers, info)
	}
	return containers
}

// RefreshContainer 刷新单个容器信息
func (m *Manager) RefreshContainer(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	var containers []DockerContainer
	if err := json.Unmarshal(output, &containers); err != nil {
		return fmt.Errorf("failed to parse container info: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("container not found")
	}

	container := containers[0]

	// 只处理Utopia管理的容器
	if container.Config.Labels["utopia.managed"] != "true" {
		return nil
	}

	claimID := container.Config.Labels["utopia.claim_id"]
	gpuIDsStr := container.Config.Labels["utopia.gpu_ids"]

	var gpuIDs []int
	if gpuIDsStr != "" {
		for _, idStr := range strings.Split(gpuIDsStr, ",") {
			if id, err := strconv.Atoi(strings.TrimSpace(idStr)); err == nil {
				gpuIDs = append(gpuIDs, id)
			}
		}
	}

	// 构建端口映射
	ports := make(map[string]string)
	for port, bindings := range container.NetworkSettings.Ports {
		if len(bindings) > 0 && bindings[0].HostPort != "" {
			ports[port] = fmt.Sprintf("%s:%s", bindings[0].HostIP, bindings[0].HostPort)
		}
	}

	// 解析时间
	created, _ := time.Parse(time.RFC3339Nano, container.Created)
	started, _ := time.Parse(time.RFC3339Nano, container.State.StartedAt)

	info := ContainerInfo{
		ID:      container.ID,
		ClaimID: claimID,
		Image:   container.Config.Image,
		Status:  container.State.Status,
		GPUIDs:  gpuIDs,
		Ports:   ports,
		Created: created.Unix(),
		Started: started.Unix(),
		Labels:  container.Config.Labels,
	}

	m.mu.Lock()
	m.containers[containerID] = info
	m.mu.Unlock()

	return nil
}

// RefreshContainers 刷新容器列表
func (m *Manager) RefreshContainers(ctx context.Context) error {
	// 列出所有容器
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "label=utopia.managed=true", "--format", "{{.ID}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	containerIDs := strings.Fields(string(output))

	m.mu.Lock()
	// 清空当前缓存
	m.containers = make(map[string]ContainerInfo)
	m.mu.Unlock()

	// 刷新每个容器的信息
	for _, id := range containerIDs {
		if err := m.RefreshContainer(ctx, id); err != nil {
			fmt.Printf("Warning: failed to refresh container %s: %v\n", id, err)
		}
	}

	return nil
}

// GetContainersByGPU 获取使用指定GPU的容器
func (m *Manager) GetContainersByGPU(gpuID int) []ContainerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ContainerInfo
	for _, info := range m.containers {
		for _, id := range info.GPUIDs {
			if id == gpuID {
				result = append(result, info)
				break
			}
		}
	}
	return result
}

// IsGPUInUse 检查GPU是否被容器使用
func (m *Manager) IsGPUInUse(gpuID int) bool {
	containers := m.GetContainersByGPU(gpuID)
	for _, container := range containers {
		// 只要有运行中的容器使用该GPU，就认为被占用
		if strings.Contains(strings.ToLower(container.Status), "running") ||
			strings.Contains(strings.ToLower(container.Status), "up") {
			return true
		}
	}
	return false
}

// 辅助函数
func convertIntSliceToStringSlice(ints []int) []string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strs
}
