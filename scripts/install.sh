#!/bin/bash

# Utopia Node Agent Installation Script
# 用于自动安装和配置Utopia节点代理

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查是否为root用户
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

# 检查依赖
check_dependencies() {
    print_info "Checking dependencies..."
    
    # 检查Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # 检查NVIDIA驱动和Docker GPU支持
    if ! nvidia-smi &> /dev/null; then
        print_warn "nvidia-smi not found. GPU monitoring may not work properly."
    fi
    
    if ! docker run --rm --gpus all nvidia/cuda:11.0-base nvidia-smi &> /dev/null; then
        print_warn "Docker GPU support not detected. Please install nvidia-docker2."
    fi
    
    print_info "Dependencies check completed"
}

# 创建用户和目录
setup_user_and_dirs() {
    print_info "Setting up user and directories..."
    
    # 创建utopia-agent用户
    if ! id "utopia-agent" &>/dev/null; then
        useradd -r -s /bin/false -G docker utopia-agent
        print_info "Created utopia-agent user"
    else
        # 确保用户在docker组中
        usermod -a -G docker utopia-agent
        print_info "utopia-agent user already exists, added to docker group"
    fi
    
    # 创建目录
    mkdir -p /etc/utopia
    mkdir -p /var/run/utopia
    mkdir -p /var/log/utopia
    
    # 设置权限
    chown utopia-agent:utopia-agent /var/run/utopia
    chown utopia-agent:utopia-agent /var/log/utopia
    chmod 755 /etc/utopia
    chmod 755 /var/run/utopia
    chmod 755 /var/log/utopia
    
    print_info "Directories created and permissions set"
}

# 安装二进制文件
install_binary() {
    print_info "Installing Utopia Node Agent binary..."
    
    if [ -f "./utopia-node-agent" ]; then
        cp ./utopia-node-agent /usr/local/bin/
        chmod +x /usr/local/bin/utopia-node-agent
        chown root:root /usr/local/bin/utopia-node-agent
        print_info "Binary installed to /usr/local/bin/utopia-node-agent"
    else
        print_error "Binary file ./utopia-node-agent not found"
        print_info "Please build the binary first with: go build -o utopia-node-agent cmd/node-agent/main.go"
        exit 1
    fi
}

# 安装配置文件
install_config() {
    print_info "Installing configuration file..."
    
    if [ -f "./configs/agent-config.yaml" ]; then
        if [ ! -f "/etc/utopia/agent-config.yaml" ]; then
            cp ./configs/agent-config.yaml /etc/utopia/
            chmod 644 /etc/utopia/agent-config.yaml
            chown root:root /etc/utopia/agent-config.yaml
            print_info "Configuration file installed to /etc/utopia/agent-config.yaml"
            print_warn "Please edit /etc/utopia/agent-config.yaml to match your environment"
        else
            print_warn "Configuration file already exists at /etc/utopia/agent-config.yaml"
            print_warn "Please review and update it manually if needed"
        fi
    else
        print_error "Configuration file ./configs/agent-config.yaml not found"
        exit 1
    fi
}

# 安装systemd服务
install_service() {
    print_info "Installing systemd service..."
    
    if [ -f "./deployments/systemd/utopia-node-agent.service" ]; then
        cp ./deployments/systemd/utopia-node-agent.service /etc/systemd/system/
        chmod 644 /etc/systemd/system/utopia-node-agent.service
        chown root:root /etc/systemd/system/utopia-node-agent.service
        
        # 重新加载systemd
        systemctl daemon-reload
        
        print_info "Systemd service installed"
        print_info "To enable and start the service, run:"
        print_info "  systemctl enable utopia-node-agent"
        print_info "  systemctl start utopia-node-agent"
    else
        print_error "Service file ./deployments/systemd/utopia-node-agent.service not found"
        exit 1
    fi
}

# 验证安装
verify_installation() {
    print_info "Verifying installation..."
    
    # 检查二进制文件
    if [ -f "/usr/local/bin/utopia-node-agent" ]; then
        print_info "✓ Binary file installed"
    else
        print_error "✗ Binary file missing"
        return 1
    fi
    
    # 检查配置文件
    if [ -f "/etc/utopia/agent-config.yaml" ]; then
        print_info "✓ Configuration file installed"
    else
        print_error "✗ Configuration file missing"
        return 1
    fi
    
    # 检查服务文件
    if [ -f "/etc/systemd/system/utopia-node-agent.service" ]; then
        print_info "✓ Systemd service installed"
    else
        print_error "✗ Systemd service missing"
        return 1
    fi
    
    # 检查用户
    if id "utopia-agent" &>/dev/null; then
        print_info "✓ utopia-agent user exists"
    else
        print_error "✗ utopia-agent user missing"
        return 1
    fi
    
    print_info "Installation verification completed successfully"
}

# 主函数
main() {
    print_info "Starting Utopia Node Agent installation..."
    
    check_root
    check_dependencies
    setup_user_and_dirs
    install_binary
    install_config
    install_service
    verify_installation
    
    print_info "Installation completed successfully!"
    print_info ""
    print_info "Next steps:"
    print_info "1. Edit /etc/utopia/agent-config.yaml with your specific configuration"
    print_info "2. Enable the service: systemctl enable utopia-node-agent"
    print_info "3. Start the service: systemctl start utopia-node-agent"
    print_info "4. Check status: systemctl status utopia-node-agent"
    print_info "5. View logs: journalctl -u utopia-node-agent -f"
}

# 运行主函数
main "$@"
