# Multi-stage build for Utopia Node Agent
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache gcc musl-dev

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o utopia-node-agent cmd/node-agent/main.go

# 运行时镜像
FROM nvidia/cuda:11.8-runtime-ubuntu20.04

# 安装必要的包
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# 创建用户
RUN groupadd -r utopia && useradd -r -g utopia utopia

# 创建目录
RUN mkdir -p /etc/utopia /var/run/utopia /var/log/utopia \
    && chown -R utopia:utopia /var/run/utopia /var/log/utopia

# 复制二进制文件
COPY --from=builder /app/utopia-node-agent /usr/local/bin/

# 复制配置文件
COPY configs/agent-config.yaml /etc/utopia/

# 设置权限
RUN chmod +x /usr/local/bin/utopia-node-agent

# 创建非root用户运行
USER utopia

# 暴露端口
EXPOSE 9200

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:9200/health || exit 1

# 启动命令
ENTRYPOINT ["/usr/local/bin/utopia-node-agent"]
CMD ["--config", "/etc/utopia/agent-config.yaml"]
