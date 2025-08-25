package frp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
)

// Config FRP配置
type Config struct {
	ServerAddr   string      `json:"server_addr"`
	ServerPort   int         `json:"server_port"`
	FrpToken     string      `json:"frp_token"`
	NodeID       string      `json:"node_id"`
	AgentApiPort int         `json:"agent_api_port"`
	Gpus         []GPUTunnel `json:"gpus"`
}

// GPUTunnel GPU隧道配置
type GPUTunnel struct {
	ID           int `json:"id"`
	WebLocalPort int `json:"web_local_port"`
	SshLocalPort int `json:"ssh_local_port"`
}

// Manager FRP管理器
type Manager struct {
	configPath string
	cmd        *exec.Cmd
	config     *Config
}

// frpc.toml模板
const frpcTemplate = `
[common]
serverAddr = "{{.ServerAddr}}"
serverPort = {{.ServerPort}}
token = "{{.FrpToken}}"
meta_node_id = "{{.NodeID}}"

# 控制隧道
[control_{{.NodeID}}]
type = "tcp"
localIP = "127.0.0.1"
localPort = {{.AgentApiPort}}
remotePort = 0
meta_tunnel_type = "agent-control"

# 数据隧道 - 使用range循环为每张卡生成
{{range .Gpus}}
[data_{{$.NodeID}}_gpu{{.ID}}_web]
type = "tcp"
localIP = "127.0.0.1"
localPort = {{.WebLocalPort}}
remotePort = 0
meta_tunnel_type = "container-data"
meta_gpu_id = {{.ID}}
meta_port_name = "web"

[data_{{$.NodeID}}_gpu{{.ID}}_ssh]
type = "tcp"
localIP = "127.0.0.1"
localPort = {{.SshLocalPort}}
remotePort = 0
meta_tunnel_type = "container-data"
meta_gpu_id = {{.ID}}
meta_port_name = "ssh"
{{end}}
`

// NewManager 创建新的FRP管理器
func NewManager(config *Config) (*Manager, error) {
	// 创建临时配置目录
	tmpDir := "/var/run/utopia"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	configPath := filepath.Join(tmpDir, "frpc.toml")

	return &Manager{
		configPath: configPath,
		config:     config,
	}, nil
}

// GenerateConfig 生成frpc配置文件
func (m *Manager) GenerateConfig() error {
	tmpl, err := template.New("frpc").Parse(frpcTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, m.config); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	log.Infof("Generated frpc config at %s", m.configPath)
	return nil
}

// Start 启动frpc进程
func (m *Manager) Start(ctx context.Context) error {
	// 首先生成配置文件
	if err := m.GenerateConfig(); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// 检查frpc是否可用
	if _, err := exec.LookPath("frpc"); err != nil {
		return fmt.Errorf("frpc not found in PATH: %w", err)
	}

	// 启动frpc进程
	m.cmd = exec.CommandContext(ctx, "frpc", "-c", m.configPath)
	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // 创建新的进程组
	}

	// 设置输出日志
	m.cmd.Stdout = log.StandardLogger().Writer()
	m.cmd.Stderr = log.StandardLogger().Writer()

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start frpc: %w", err)
	}

	log.Infof("Started frpc process (PID: %d)", m.cmd.Process.Pid)

	// 等待一小段时间确保frpc启动成功
	time.Sleep(2 * time.Second)

	// 检查进程是否还在运行
	if m.cmd.Process != nil {
		if err := m.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return fmt.Errorf("frpc process failed to start properly: %w", err)
		}
	}

	return nil
}

// Stop 停止frpc进程
func (m *Manager) Stop() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	log.Info("Stopping frpc process...")

	// 发送SIGTERM信号
	if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Warnf("Failed to send SIGTERM to frpc: %v", err)
	}

	// 等待进程退出
	done := make(chan error, 1)
	go func() {
		done <- m.cmd.Wait()
	}()

	select {
	case err := <-done:
		log.Info("frpc process stopped gracefully")
		return err
	case <-time.After(10 * time.Second):
		// 超时后强制杀死进程
		log.Warn("frpc process did not stop gracefully, force killing...")
		if err := m.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill frpc process: %w", err)
		}
		<-done // 等待Wait()返回
		log.Info("frpc process killed")
		return nil
	}
}

// IsRunning 检查frpc是否在运行
func (m *Manager) IsRunning() bool {
	if m.cmd == nil || m.cmd.Process == nil {
		return false
	}

	// 发送信号0检查进程是否存在
	err := m.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// Restart 重启frpc进程
func (m *Manager) Restart(ctx context.Context) error {
	log.Info("Restarting frpc process...")

	if err := m.Stop(); err != nil {
		log.Warnf("Error stopping frpc: %v", err)
	}

	// 等待一下再启动
	time.Sleep(1 * time.Second)

	return m.Start(ctx)
}

// GetPID 获取frpc进程ID
func (m *Manager) GetPID() int {
	if m.cmd == nil || m.cmd.Process == nil {
		return 0
	}
	return m.cmd.Process.Pid
}

// UpdateConfig 更新配置并重启
func (m *Manager) UpdateConfig(ctx context.Context, config *Config) error {
	m.config = config
	return m.Restart(ctx)
}

// CleanupConfig 清理配置文件
func (m *Manager) CleanupConfig() error {
	if m.configPath != "" {
		if err := os.Remove(m.configPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove config file: %w", err)
		}
	}
	return nil
}
