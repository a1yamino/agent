package registration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RegisterRequest 注册请求
type RegisterRequest struct {
	MachineID      string `json:"machine_id"`
	BootstrapToken string `json:"bootstrap_token,omitempty"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	NodeID    string `json:"node_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// Client 注册客户端
type Client struct {
	apiURL     string
	httpClient *http.Client
}

// NewClient 创建新的注册客户端
func NewClient(apiURL string) *Client {
	return &Client{
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetMachineID 获取机器ID
func GetMachineID() (string, error) {
	// 尝试从 /etc/machine-id 读取
	machineID, err := readMachineIDFromFile("/etc/machine-id")
	if err == nil && machineID != "" {
		return machineID, nil
	}

	// 尝试从 /var/lib/dbus/machine-id 读取
	machineID, err = readMachineIDFromFile("/var/lib/dbus/machine-id")
	if err == nil && machineID != "" {
		return machineID, nil
	}

	return "", fmt.Errorf("failed to read machine ID")
}

// readMachineIDFromFile 从文件读取机器ID
func readMachineIDFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	machineID := strings.TrimSpace(string(data))
	if machineID == "" {
		return "", fmt.Errorf("empty machine ID")
	}

	return machineID, nil
}

// LoadNodeID 从文件加载节点ID
func LoadNodeID(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // 文件不存在，返回空字符串
		}
		return "", fmt.Errorf("failed to read node ID file: %w", err)
	}

	nodeID := strings.TrimSpace(string(data))
	return nodeID, nil
}

// SaveNodeID 保存节点ID到文件
func SaveNodeID(filePath, nodeID string) error {
	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 原子写入
	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(nodeID), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filePath); err != nil {
		os.Remove(tmpFile) // 清理临时文件
		return fmt.Errorf("failed to move temp file: %w", err)
	}

	return nil
}

// Register 向中央平台注册节点
func (c *Client) Register(machineID, bootstrapToken string) (*RegisterResponse, error) {
	req := RegisterRequest{
		MachineID:      machineID,
		BootstrapToken: bootstrapToken,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.apiURL+"/api/v1/nodes/register",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var registerResp RegisterResponse
	if err := json.Unmarshal(body, &registerResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &registerResp, nil
}
