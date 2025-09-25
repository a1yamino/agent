package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 节点代理配置
type Config struct {
	// 节点ID持久化路径
	IdentityFilePath string `yaml:"identity_file_path"`

	// 中央平台信息
	CentralPlatform CentralPlatformConfig `yaml:"central_platform"`

	// FRP相关配置
	FRP FRPConfig `yaml:"frp"`

	// Agent自身API服务配置
	AgentAPI AgentAPIConfig `yaml:"agent_api"`
}

// CentralPlatformConfig 中央平台配置
type CentralPlatformConfig struct {
	APIURL         string `yaml:"api_url"`
	BootstrapToken string `yaml:"bootstrap_token,omitempty"`
}

// FRPConfig FRP配置
type FRPConfig struct {
	ServerAddr string `yaml:"server_addr"`
	ServerPort int    `yaml:"server_port"`
	Token      string `yaml:"token"`
}

// AgentAPIConfig Agent API配置
type AgentAPIConfig struct {
	ListenAddress string `yaml:"listen_address"`
	AuthToken     string `yaml:"auth_token"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		IdentityFilePath: "/etc/utopia/node_id",
		CentralPlatform: CentralPlatformConfig{
			APIURL: "http://api.server.com",
		},
		FRP: FRPConfig{
			ServerAddr: "api.server.com",
			ServerPort: 7000,
			Token:      "frp_connection_token",
		},
		AgentAPI: AgentAPIConfig{
			ListenAddress: "127.0.0.1:9200",
			AuthToken:     "a_very_secret_agent_api_token",
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	// 如果配置文件不存在，返回默认配置
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.IdentityFilePath = os.ExpandEnv(cfg.IdentityFilePath)
	return cfg, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.CentralPlatform.APIURL == "" {
		return fmt.Errorf("central_platform.api_url is required")
	}
	if c.FRP.ServerAddr == "" {
		return fmt.Errorf("frp.server_addr is required")
	}
	if c.FRP.ServerPort <= 0 {
		return fmt.Errorf("frp.server_port must be positive")
	}
	if c.AgentAPI.ListenAddress == "" {
		return fmt.Errorf("agent_api.listen_address is required")
	}
	if c.AgentAPI.AuthToken == "" {
		return fmt.Errorf("agent_api.auth_token is required")
	}
	return nil
}
