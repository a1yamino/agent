package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"utopia-node-agent/internal/container"
	"utopia-node-agent/internal/gpu"
	"utopia-node-agent/internal/system"

	"github.com/gin-gonic/gin"
)

// Server API服务器
type Server struct {
	engine           *gin.Engine
	server           *http.Server
	containerManager *container.Manager
	gpuMonitor       *gpu.Monitor
	systemMonitor    *system.Monitor
	authToken        string
}

// MetricsResponse 指标响应
type MetricsResponse struct {
	NodeID             string                `json:"node_id"`
	CPUUsagePercent    float64               `json:"cpu_usage_percent"`
	MemoryUsagePercent float64               `json:"memory_usage_percent"`
	GPUs               []gpu.GPUInfo         `json:"gpus"`
	System             *system.SystemMetrics `json:"system,omitempty"`
}

// CreateContainerResponse 创建容器响应
type CreateContainerResponse struct {
	ContainerID string `json:"container_id"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// NewServer 创建新的API服务器
func NewServer(
	containerManager *container.Manager,
	gpuMonitor *gpu.Monitor,
	systemMonitor *system.Monitor,
	authToken string,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// 添加中间件
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	server := &Server{
		engine:           engine,
		containerManager: containerManager,
		gpuMonitor:       gpuMonitor,
		systemMonitor:    systemMonitor,
		authToken:        authToken,
	}

	// 设置路由
	server.setupRoutes()

	return server
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 认证中间件
	authMiddleware := s.authMiddleware()

	// API v1 路由组
	v1 := s.engine.Group("/api/v1")
	v1.Use(authMiddleware)

	// 容器管理
	v1.POST("/containers", s.createContainer)
	v1.DELETE("/containers/:id", s.removeContainer)
	v1.GET("/containers", s.listContainers)
	v1.GET("/containers/:id", s.getContainer)

	// 系统指标
	v1.GET("/metrics", s.getMetrics)

	// 健康检查（不需要认证）
	s.engine.GET("/health", s.healthCheck)
}

// authMiddleware 认证中间件
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error: "Authorization header required",
				Code:  401,
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error: "Invalid authorization header format",
				Code:  401,
			})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != s.authToken {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error: "Invalid token",
				Code:  401,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// createContainer 创建容器
func (s *Server) createContainer(c *gin.Context) {
	var req container.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    400,
			Details: err.Error(),
		})
		return
	}

	// 验证GPU数量是否合理
	if req.GPUCount < 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "GPU count must be non-negative",
			Code:  400,
		})
		return
	}

	// 检查是否有足够的可用GPU
	availableGPUs := s.gpuMonitor.GetAvailableGPUs()
	if req.GPUCount > len(availableGPUs) {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: fmt.Sprintf("Not enough available GPUs: requested %d, available %d", req.GPUCount, len(availableGPUs)),
			Code:  409,
		})
		return
	}

	// 创建容器
	ctx := context.Background()
	containerID, err := s.containerManager.CreateContainer(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create container",
			Code:    500,
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, CreateContainerResponse{
		ContainerID: containerID,
	})
}

// removeContainer 删除容器
func (s *Server) removeContainer(c *gin.Context) {
	containerID := c.Param("id")
	if containerID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Container ID is required",
			Code:  400,
		})
		return
	}

	ctx := context.Background()
	if err := s.containerManager.RemoveContainer(ctx, containerID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to remove container",
			Code:    500,
			Details: err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// listContainers 列出容器
func (s *Server) listContainers(c *gin.Context) {
	containers := s.containerManager.ListContainers()
	c.JSON(http.StatusOK, containers)
}

// getContainer 获取容器信息
func (s *Server) getContainer(c *gin.Context) {
	containerID := c.Param("id")
	if containerID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Container ID is required",
			Code:  400,
		})
		return
	}

	container, exists := s.containerManager.GetContainer(containerID)
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Container not found",
			Code:  404,
		})
		return
	}

	c.JSON(http.StatusOK, container)
}

// getMetrics 获取系统指标
func (s *Server) getMetrics(c *gin.Context) {
	// 刷新GPU信息
	if err := s.gpuMonitor.RefreshGPUInfo(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to refresh GPU info",
			Code:    500,
			Details: err.Error(),
		})
		return
	}

	// 获取GPU信息
	gpus := s.gpuMonitor.GetGPUInfo()

	// 获取系统指标
	systemMetrics, err := s.systemMonitor.GetSystemMetrics()
	if err != nil {
		// 系统指标获取失败不影响GPU指标返回
		systemMetrics = &system.SystemMetrics{}
	}

	// 获取节点ID（从查询参数或配置中获取）
	nodeID := c.Query("node_id")
	if nodeID == "" {
		nodeID = "unknown"
	}

	response := MetricsResponse{
		NodeID:             nodeID,
		CPUUsagePercent:    systemMetrics.CPUUsagePercent,
		MemoryUsagePercent: systemMetrics.MemoryUsagePercent,
		GPUs:               gpus,
		System:             systemMetrics,
	}

	c.JSON(http.StatusOK, response)
}

// healthCheck 健康检查
func (s *Server) healthCheck(c *gin.Context) {
	// 检查GPU监控器
	if _, err := s.gpuMonitor.GetGPUCount(); err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "GPU monitor not available",
			Code:    503,
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": c.GetHeader("X-Request-Time"),
	})
}

// Start 启动服务器
func (s *Server) Start(address string) error {
	s.server = &http.Server{
		Addr:    address,
		Handler: s.engine,
	}

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	return s.server.Shutdown(ctx)
}
