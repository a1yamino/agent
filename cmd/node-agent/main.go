package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"utopia-node-agent/internal/agent"
	"utopia-node-agent/internal/config"

	log "github.com/sirupsen/logrus"
)

var (
	version = "1.0.0"
	commit  = "dev"
)

func main() {
	var (
		configPath  = flag.String("config", "/etc/utopia/agent-config.yaml", "Configuration file path")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Utopia Node Agent v%s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	// 配置日志
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建并启动代理
	nodeAgent, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动代理
	errChan := make(chan error, 1)
	go func() {
		if err := nodeAgent.Start(); err != nil {
			errChan <- err
		}
	}()

	log.Info("Utopia Node Agent started successfully")

	// 等待信号或错误
	select {
	case err := <-errChan:
		log.Errorf("Agent error: %v", err)
	case sig := <-sigChan:
		log.Infof("Received signal: %v", sig)
	}

	// 优雅关闭
	log.Info("Shutting down...")

	// 设置关闭超时
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- nodeAgent.Stop()
	}()

	// 监听第二个 Ctrl+C 信号以强制退出
	go func() {
		<-sigChan
		log.Warn("Received second signal, forcing exit...")
		os.Exit(1)
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			log.Errorf("Error during shutdown: %v", err)
		} else {
			log.Info("Shutdown completed successfully")
		}
	case <-time.After(20 * time.Second):
		log.Error("Shutdown timeout exceeded, forcing exit...")
		os.Exit(1)
	}
}
