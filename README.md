# Utopia Node Agent

Utopia Platform GPU节点智能代理，用于管理GPU服务器上的容器生命周期、GPU资源监控以及与中央平台的通信。

## 特性

- **自主注册**: 首次启动时自动向中央平台注册获取节点身份
- **GPU监控**: 使用go-nvml实时监控GPU状态和使用情况
- **容器管理**: 通过API管理Docker容器的创建、删除和监控
- **FRP隧道**: 自动配置和管理frp隧道，实现安全的远程访问
- **系统监控**: 监控CPU、内存等系统资源使用情况
- **RESTful API**: 提供完整的REST API接口
- **安全通信**: 所有通信通过认证令牌和FRP隧道加密

## 架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Central       │    │   FRP Server    │    │   Node Agent    │
│   Platform      │◄──►│                 │◄──►│                 │
│                 │    │                 │    │  ┌─────────────┐ │
└─────────────────┘    └─────────────────┘    │  │ Container   │ │
                                               │  │ Manager     │ │
                                               │  └─────────────┘ │
                                               │  ┌─────────────┐ │
                                               │  │ GPU Monitor │ │
                                               │  └─────────────┘ │
                                               │  ┌─────────────┐ │
                                               │  │ API Server  │ │
                                               │  └─────────────┘ │
                                               └─────────────────┘
```

## 快速开始

### 前置要求

- Linux操作系统
- Docker Engine
- NVIDIA GPU + 驱动程序
- nvidia-docker2（用于GPU容器支持）
- frpc客户端程序

### 安装

1. **下载并构建**

```bash
git clone <repository-url>
cd utopia-node-agent
make build
```

2. **系统安装**

```bash
# 使用安装脚本（推荐）
sudo make system-install

# 或手动安装
sudo cp build/utopia-node-agent /usr/local/bin/
sudo cp configs/agent-config.yaml /etc/utopia/
sudo cp deployments/systemd/utopia-node-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
```

3. **配置**

编辑配置文件 `/etc/utopia/agent-config.yaml`:

```yaml
# 中央平台信息
central_platform:
  api_url: "http://your-platform-api.com"
  
# FRP配置
frp:
  server_addr: "your-frp-server.com"
  server_port: 7000
  token: "your-frp-token"

# Agent API配置
agent_api:
  listen_address: "127.0.0.1:9200"
  auth_token: "your-secret-token"
```

4. **启动服务**

```bash
sudo systemctl enable utopia-node-agent
sudo systemctl start utopia-node-agent
```

### 验证安装

```bash
# 检查服务状态
sudo systemctl status utopia-node-agent

# 查看日志
sudo journalctl -u utopia-node-agent -f

# 测试API（需要在FRP隧道建立后通过平台访问）
curl -H "Authorization: Bearer your-secret-token" \
     http://localhost:9200/api/v1/metrics
```

## API文档

### 认证

所有API请求需要在Header中包含认证令牌：

```
Authorization: Bearer <your-auth-token>
```

### 端点

#### 容器管理

**创建容器**
```http
POST /api/v1/containers
Content-Type: application/json

{
  "claim_id": "claim-123",
  "image": "nvidia/cuda:11.8-runtime-ubuntu20.04",
  "gpu_ids": [0, 1],
  "port_mappings": [
    {
      "host_port": 8888,
      "container_port": 8888
    }
  ],
  "env_vars": ["NVIDIA_VISIBLE_DEVICES=0,1"],
  "command": ["/bin/bash", "-c", "sleep infinity"]
}
```

**删除容器**
```http
DELETE /api/v1/containers/{container_id}
```

**列出容器**
```http
GET /api/v1/containers
```

#### 系统监控

**获取系统指标**
```http
GET /api/v1/metrics
```

响应示例：
```json
{
  "node_id": "node-abc123",
  "cpu_usage_percent": 25.5,
  "memory_usage_percent": 60.2,
  "gpus": [
    {
      "id": 0,
      "temperature_c": 65,
      "memory_total_mb": 24576,
      "memory_used_mb": 1024,
      "name": "NVIDIA RTX 4090",
      "usage_percent": 15.0,
      "busy": false
    }
  ]
}
```

#### 健康检查

```http
GET /health
```

## 配置说明

### 配置文件结构

```yaml
# 节点ID持久化路径
identity_file_path: "/etc/utopia/node_id"

# 中央平台配置
central_platform:
  api_url: "http://api.server.com"
  bootstrap_token: "optional-bootstrap-token"

# FRP配置
frp:
  server_addr: "frp.server.com"
  server_port: 7000
  token: "frp-auth-token"

# Agent API配置
agent_api:
  listen_address: "127.0.0.1:9200"
  auth_token: "api-auth-token"
```

### 环境变量

可以通过环境变量覆盖配置：

- `PHOENIX_API_URL`: 中央平台API地址
- `PHOENIX_FRP_SERVER`: FRP服务器地址
- `PHOENIX_FRP_TOKEN`: FRP认证令牌
- `PHOENIX_AUTH_TOKEN`: API认证令牌

## 开发

### 构建

```bash
# 开发构建
make build

# Linux构建
make build-linux

# 运行测试
make test

# 代码格式化
make fmt

# 代码检查
make vet
```

### 开发模式运行

```bash
make run
```

### Docker构建

```bash
make docker-build
```

## 监控和日志

### 系统日志

```bash
# 查看服务日志
sudo journalctl -u utopia-node-agent -f

# 查看最近的错误
sudo journalctl -u utopia-node-agent --since "1 hour ago" -p err
```

### 关键日志位置

- 服务日志: `journalctl -u utopia-node-agent`
- FRP日志: 通过Agent服务日志查看
- Agent配置: `/etc/utopia/agent-config.yaml`
- 节点ID: `/etc/utopia/node_id`

## 故障排除

### 常见问题

1. **GPU监控失败**
   - 检查NVIDIA驱动安装: `nvidia-smi`
   - 检查NVML库: `ldconfig -p | grep nvml`

2. **容器创建失败**
   - 检查Docker服务: `systemctl status docker`
   - 检查GPU支持: `docker run --rm --gpus all nvidia/cuda:11.0-base nvidia-smi`

3. **FRP连接失败**
   - 检查网络连接到FRP服务器
   - 验证FRP令牌配置
   - 检查防火墙设置

4. **注册失败**
   - 检查中央平台API可达性
   - 验证引导令牌（如果需要）

### 日志级别

修改日志级别以获取更多调试信息：

```bash
# 在配置文件中添加
log_level: "debug"
```

## 安全注意事项

1. **认证令牌**: 使用强随机令牌并定期轮换
2. **网络安全**: Agent API仅监听本地回环地址
3. **文件权限**: 确保配置文件权限适当
4. **FRP安全**: 使用安全的FRP令牌和TLS连接

## 许可证

[许可证信息]

## 贡献

[贡献指南]

## 支持

如有问题，请提交Issue或联系维护团队。
