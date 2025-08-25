# Utopia Node Agent Makefile

# 变量定义
BINARY_NAME=utopia-node-agent
MAIN_PATH=cmd/node-agent/main.go
BUILD_DIR=build
VERSION?=1.0.0
COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go变量
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# 构建标志
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# 默认目标
.PHONY: all
all: clean build

# 构建
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# 构建Linux版本
.PHONY: build-linux
build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "Linux build completed: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

# 清理
.PHONY: clean
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "Clean completed"

# 测试
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# 格式化代码
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@$(GOCMD) fmt ./...

# 检查代码
.PHONY: vet
vet:
	@echo "Vetting code..."
	@$(GOCMD) vet ./...

# 下载依赖
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy

# 安装到本地
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installation completed"

# 卸载
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstallation completed"

# 运行开发版本
.PHONY: run
run:
	@echo "Running $(BINARY_NAME) in development mode..."
	@$(GOCMD) run $(MAIN_PATH) --config configs/agent-config.yaml

# 构建Docker镜像
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t utopia-node-agent:$(VERSION) .
	docker build -t utopia-node-agent:latest .

# 系统安装（使用安装脚本）
.PHONY: system-install
system-install: build
	@echo "Running system installation..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) ./utopia-node-agent
	@sudo bash scripts/install.sh
	@rm -f ./utopia-node-agent

# 创建发布包
.PHONY: package
package: build-linux
	@echo "Creating release package..."
	@mkdir -p $(BUILD_DIR)/package
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BUILD_DIR)/package/utopia-node-agent
	@cp -r configs $(BUILD_DIR)/package/
	@cp -r deployments $(BUILD_DIR)/package/
	@cp -r scripts $(BUILD_DIR)/package/
	@cp README.md $(BUILD_DIR)/package/
	@cd $(BUILD_DIR) && tar -czf utopia-node-agent-$(VERSION)-linux-amd64.tar.gz package/
	@echo "Package created: $(BUILD_DIR)/utopia-node-agent-$(VERSION)-linux-amd64.tar.gz"

# 显示帮助
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  build-linux   - Build for Linux"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code"
	@echo "  deps          - Download dependencies"
	@echo "  install       - Install binary to /usr/local/bin"
	@echo "  uninstall     - Remove binary from /usr/local/bin"
	@echo "  run           - Run in development mode"
	@echo "  docker-build  - Build Docker image"
	@echo "  system-install- Install as system service"
	@echo "  package       - Create release package"
	@echo "  help          - Show this help"
