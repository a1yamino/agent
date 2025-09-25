package agent

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"utopia-node-agent/internal/api"
	"utopia-node-agent/internal/config"
	"utopia-node-agent/internal/container"
	"utopia-node-agent/internal/frp"
	"utopia-node-agent/internal/gpu"
	"utopia-node-agent/internal/registration"
	"utopia-node-agent/internal/system"
)

// Agent 节点代理
type Agent struct {
	config           *config.Config
	nodeID           string
	containerManager *container.Manager
	gpuMonitor       *gpu.Monitor
	systemMonitor    *system.Monitor
	frpManager       *frp.Manager
	apiServer        *api.Server
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	mu               sync.RWMutex
}

// New 创建新的代理实例
func New(cfg *config.Config) (*Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	return agent, nil
}

// Start 启动代理
func (a *Agent) Start() error {
	// 1. 启动与注册工作流
	if err := a.bootstrap(); err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// 2. 初始化监控器
	if err := a.initializeMonitors(); err != nil {
		return fmt.Errorf("failed to initialize monitors: %w", err)
	}

	// 3. 初始化容器管理器
	if err := a.initializeContainerManager(); err != nil {
		return fmt.Errorf("failed to initialize container manager: %w", err)
	}

	// 4. 启动FRP管理器
	if err := a.startFRP(); err != nil {
		return fmt.Errorf("failed to start FRP: %w", err)
	}

	// 5. 启动API服务器
	if err := a.startAPIServer(); err != nil {
		return fmt.Errorf("failed to start API server: %w", err)
	}

	// 6. 启动后台任务
	a.startBackgroundTasks()

	return nil
}

// Stop 停止代理
func (a *Agent) Stop() error {
	fmt.Println("Stopping Utopia Node Agent...")

	// 取消上下文
	a.cancel()

	// 等待所有goroutine完成，但设置超时
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All goroutines stopped gracefully")
	case <-time.After(15 * time.Second):
		fmt.Println("Warning: Timeout waiting for goroutines to stop")
	}

	// 停止API服务器
	if a.apiServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.apiServer.Stop(ctx); err != nil {
			fmt.Printf("Error stopping API server: %v\n", err)
		} else {
			fmt.Println("API server stopped")
		}
	}

	// 停止FRP
	if a.frpManager != nil {
		if err := a.frpManager.Stop(); err != nil {
			fmt.Printf("Error stopping FRP: %v\n", err)
		} else {
			fmt.Println("FRP stopped")
		}
		if err := a.frpManager.CleanupConfig(); err != nil {
			fmt.Printf("Error cleaning up FRP config: %v\n", err)
		}
	}

	// 关闭监控器
	if a.gpuMonitor != nil {
		if err := a.gpuMonitor.Close(); err != nil {
			fmt.Printf("Error closing GPU monitor: %v\n", err)
		} else {
			fmt.Println("GPU monitor closed")
		}
	}

	// 关闭容器管理器
	if a.containerManager != nil {
		if err := a.containerManager.Close(); err != nil {
			fmt.Printf("Error closing container manager: %v\n", err)
		} else {
			fmt.Println("Container manager closed")
		}
	}

	fmt.Println("Utopia Node Agent stopped")
	return nil
}

// bootstrap 启动与注册工作流
func (a *Agent) bootstrap() error {
	// 1. 检查本地身份
	log.Printf("Checking for existing node ID at %s...", a.config.IdentityFilePath)
	nodeID, err := registration.LoadNodeID(a.config.IdentityFilePath)
	if err != nil {
		return fmt.Errorf("failed to load node ID: %w", err)
	}

	if nodeID != "" {
		a.nodeID = nodeID
		fmt.Printf("Loaded existing node ID: %s\n", nodeID)
		return nil
	}

	// // 2. 获取机器ID
	// machineID, err := registration.GetMachineID()
	// if err != nil {
	// 	return fmt.Errorf("failed to get machine ID: %w", err)
	// }
	// fmt.Printf("Machine ID: %s\n", machineID)

	hostName, err := registration.GetHostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	fmt.Printf("Hostname: %s\n", hostName)

	// 3. 向平台注册
	regClient := registration.NewClient(a.config.CentralPlatform.APIURL)
	regResp, err := regClient.Register(a.config.CentralPlatform.BootstrapToken, hostName)
	if err != nil {
		return fmt.Errorf("failed to register with platform: %w", err)
	}

	// 4. 持久化身份
	if err := registration.SaveNodeID(a.config.IdentityFilePath, regResp.NodeID); err != nil {
		return fmt.Errorf("failed to save node ID: %w", err)
	}

	a.nodeID = strconv.FormatInt(regResp.NodeID, 10)
	fmt.Printf("Successfully registered as node: %d\n", regResp.NodeID)

	return nil
}

// initializeMonitors 初始化监控器
func (a *Agent) initializeMonitors() error {
	// 初始化GPU监控器
	gpuMonitor, err := gpu.NewMonitor()
	if err != nil {
		return fmt.Errorf("failed to create GPU monitor: %w", err)
	}
	a.gpuMonitor = gpuMonitor

	// 初始化系统监控器
	a.systemMonitor = system.NewMonitor()

	// 刷新一次GPU信息
	if err := a.gpuMonitor.RefreshGPUInfo(); err != nil {
		return fmt.Errorf("failed to refresh GPU info: %w", err)
	}

	gpuCount, err := a.gpuMonitor.GetGPUCount()
	if err != nil {
		return fmt.Errorf("failed to get GPU count: %w", err)
	}

	fmt.Printf("Detected %d GPU(s)\n", gpuCount)

	return nil
}

// initializeContainerManager 初始化容器管理器
func (a *Agent) initializeContainerManager() error {
	containerManager, err := container.NewManager(a.gpuMonitor)
	if err != nil {
		return fmt.Errorf("failed to create container manager: %w", err)
	}
	a.containerManager = containerManager

	// 刷新现有容器
	if err := a.containerManager.RefreshContainers(a.ctx); err != nil {
		fmt.Printf("Warning: failed to refresh existing containers: %v\n", err)
	}

	return nil
}

// startFRP 启动FRP管理器
func (a *Agent) startFRP() error {
	// 生成FRP配置
	frpConfig := a.generateFRPConfig()

	// 创建FRP管理器
	frpManager, err := frp.NewManager(frpConfig)
	if err != nil {
		return fmt.Errorf("failed to create FRP manager: %w", err)
	}
	a.frpManager = frpManager

	// 启动FRP
	if err := a.frpManager.Start(a.ctx); err != nil {
		return fmt.Errorf("failed to start FRP: %w", err)
	}

	fmt.Printf("FRP started (PID: %d)\n", a.frpManager.GetPID())

	return nil
}

// generateFRPConfig 生成FRP配置
func (a *Agent) generateFRPConfig() *frp.Config {
	// 解析Agent API端口
	apiPort := 9200
	if portStr := getPortFromAddress(a.config.AgentAPI.ListenAddress); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			apiPort = port
		}
	}

	// 生成GPU隧道配置
	gpuCount, _ := a.gpuMonitor.GetGPUCount()
	var gpuTunnels []frp.GPUTunnel

	for i := 0; i < gpuCount; i++ {
		gpuTunnels = append(gpuTunnels, frp.GPUTunnel{
			ID:           i,
			WebLocalPort: 8000 + i*10,     // 为每个GPU分配Web端口范围
			SshLocalPort: 8000 + i*10 + 1, // SSH端口
		})
	}

	return &frp.Config{
		ServerAddr:   a.config.FRP.ServerAddr,
		ServerPort:   a.config.FRP.ServerPort,
		FrpToken:     a.config.FRP.Token,
		NodeID:       a.nodeID,
		AgentApiPort: apiPort,
		Gpus:         gpuTunnels,
	}
}

// startAPIServer 启动API服务器
func (a *Agent) startAPIServer() error {
	// 创建API服务器
	a.apiServer = api.NewServer(
		a.containerManager,
		a.gpuMonitor,
		a.systemMonitor,
		a.config.AgentAPI.AuthToken,
	)

	// 在后台启动服务器
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.apiServer.Start(a.config.AgentAPI.ListenAddress); err != nil {
			fmt.Printf("API server error: %v\n", err)
		}
	}()

	// 等待一下确保服务器启动
	time.Sleep(1 * time.Second)

	fmt.Printf("API server started on %s\n", a.config.AgentAPI.ListenAddress)

	return nil
}

// startBackgroundTasks 启动后台任务
func (a *Agent) startBackgroundTasks() {
	// 启动GPU监控任务
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.gpuMonitorTask()
	}()

	// 启动容器监控任务
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.containerMonitorTask()
	}()

	// 启动FRP监控任务
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.frpMonitorTask()
	}()
}

// gpuMonitorTask GPU监控任务
func (a *Agent) gpuMonitorTask() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.gpuMonitor.RefreshGPUInfo(); err != nil {
				fmt.Printf("Failed to refresh GPU info: %v\n", err)
			}
		}
	}
}

// containerMonitorTask 容器监控任务
func (a *Agent) containerMonitorTask() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.containerManager.RefreshContainers(a.ctx); err != nil {
				fmt.Printf("Failed to refresh containers: %v\n", err)
			}
		}
	}
}

// frpMonitorTask FRP监控任务
func (a *Agent) frpMonitorTask() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if !a.frpManager.IsRunning() {
				fmt.Println("FRP process died, restarting...")
				if err := a.frpManager.Restart(a.ctx); err != nil {
					fmt.Printf("Failed to restart FRP: %v\n", err)
				} else {
					fmt.Println("FRP restarted successfully")
				}
			}
		}
	}
}

// getPortFromAddress 从地址中提取端口
func getPortFromAddress(address string) string {
	parts := strings.Split(address, ":")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
