package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"shop_server/config"
	"shop_server/internal/service"

	"github.com/gin-gonic/gin"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	config  *config.Config
	server  *http.Server
	router  *gin.Engine
	manager *service.Manager
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(cfg *config.Config, manager *service.Manager) *HTTPServer {
	// 设置gin模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// 添加中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	httpServer := &HTTPServer{
		config:  cfg,
		router:  router,
		manager: manager,
	}

	// 设置路由
	httpServer.setupRoutes()

	// 创建HTTP服务器
	httpServer.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return httpServer
}

// setupRoutes 设置路由
func (h *HTTPServer) setupRoutes() {
	// 健康检查
	h.router.GET("/health", h.healthCheck)
	h.router.GET("/", h.index)

	// API路由组
	api := h.router.Group("/api")
	{
		// 门禁设备推送接收端点
		api.POST("/plate/station/:station_id", h.handlePlateMessage)

		// 门禁设备心跳注册端点
		api.POST("/device/heartbeat/:station_id", h.handleDeviceHeartbeat)

		// 门禁设备截图上传端点
		api.POST("/device/snapshot/:station_id", h.handleSnapshot)

		// 状态查询端点
		api.GET("/status", h.getSystemStatus)
		api.GET("/status/station/:station_id", h.getStationStatus)

		// 测试端点
		api.POST("/test/mqtt", h.testMQTTPublish)
	}

	log.Printf("HTTP路由已设置，等待服务器初始化...")
}

// Start 启动HTTP服务器
func (h *HTTPServer) Start(ctx context.Context) error {
	log.Printf("启动HTTP服务器: %s", h.server.Addr)

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器启动失败: %v", err)
		}
	}()

	log.Printf("HTTP服务器已启动: %s", h.server.Addr)
	return nil
}

// Stop 停止HTTP服务器
func (h *HTTPServer) Stop(ctx context.Context) error {
	log.Println("停止HTTP服务器...")

	// 设置关闭超时
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := h.server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP服务器关闭失败: %v", err)
		return err
	}

	log.Println("HTTP服务器已停止")
	return nil
}

// healthCheck 健康检查
func (h *HTTPServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"service":   "shop_server",
		"shop_id":   h.config.Shop.ID,
	})
}

// index 首页
func (h *HTTPServer) index(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":   "Shop Server - 洗车店智能设备管理系统",
		"shop_id":   h.config.Shop.ID,
		"shop_name": h.config.Shop.Name,
		"version":   "1.0.0",
		"endpoints": []string{
			"/health - 健康检查",
			"/api/plate/station/{station_id} - 门禁推送接收",
			"/api/device/heartbeat/{station_id} - 设备心跳",
			"/api/status - 系统状态",
		},
	})
}

// handlePlateMessage 处理门禁设备推送消息
func (h *HTTPServer) handlePlateMessage(c *gin.Context) {
	stationID := c.Param("station_id")

	log.Printf("收到门禁设备推送: 工位=%s, IP=%s", stationID, c.ClientIP())

	// 验证工位是否存在
	if _, err := h.config.GetStationByID(stationID); err != nil {
		log.Printf("无效的工位ID: %s", stationID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的工位ID: %s", stationID),
		})
		return
	}

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("读取请求体失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取请求体失败",
		})
		return
	}

	log.Printf("门禁推送内容: %s", string(body))

	// 解析门禁消息
	plateMsg, err := h.parsePlateMessage(body)
	if err != nil {
		log.Printf("解析门禁消息失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析消息失败",
		})
		return
	}

	// 处理门禁消息
	response, err := h.processPlateMessage(stationID, plateMsg)
	if err != nil {
		log.Printf("处理门禁消息失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理消息失败",
		})
		return
	}

	// 返回响应
	if response != nil {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}

// handleDeviceHeartbeat 处理设备心跳
func (h *HTTPServer) handleDeviceHeartbeat(c *gin.Context) {
	stationID := c.Param("station_id")

	log.Printf("收到设备心跳: 工位=%s, IP=%s", stationID, c.ClientIP())

	// 更新设备状态
	h.manager.GetDeviceManager().UpdateDeviceStatus(stationID, &service.DeviceStatus{
		DeviceType: service.DeviceTypePlate,
		StationID:  stationID,
		Online:     true,
		LastSeen:   time.Now(),
		Status:     "online",
		Data: map[string]interface{}{
			"ip_address": c.ClientIP(),
			"user_agent": c.GetHeader("User-Agent"),
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "心跳已接收",
	})
}

// handleSnapshot 处理截图上传
func (h *HTTPServer) handleSnapshot(c *gin.Context) {
	stationID := c.Param("station_id")

	log.Printf("收到截图上传: 工位=%s, IP=%s", stationID, c.ClientIP())

	// 读取截图数据
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("读取截图数据失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取截图数据失败",
		})
		return
	}

	// 处理截图数据（这里可以保存到文件系统或云存储）
	log.Printf("收到截图数据: 工位=%s, 大小=%d bytes", stationID, len(body))

	// 发布截图事件到MQTT
	h.publishSnapshotEvent(stationID, len(body))

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "截图已接收",
		"size":    len(body),
	})
}

// getSystemStatus 获取系统状态
func (h *HTTPServer) getSystemStatus(c *gin.Context) {
	deviceManager := h.manager.GetDeviceManager()
	allStatus := deviceManager.GetAllDeviceStatus()

	c.JSON(http.StatusOK, gin.H{
		"shop_id":      h.config.Shop.ID,
		"shop_name":    h.config.Shop.Name,
		"timestamp":    time.Now().Unix(),
		"device_count": len(allStatus),
		"devices":      allStatus,
	})
}

// getStationStatus 获取工位状态
func (h *HTTPServer) getStationStatus(c *gin.Context) {
	stationID := c.Param("station_id")

	deviceManager := h.manager.GetDeviceManager()
	status, err := deviceManager.GetDeviceStatus(stationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"station_id": stationID,
		"status":     status,
		"timestamp":  time.Now().Unix(),
	})
}

// testMQTTPublish 测试MQTT发布
func (h *HTTPServer) testMQTTPublish(c *gin.Context) {
	var req struct {
		StationID string      `json:"station_id"`
		Message   interface{} `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式",
		})
		return
	}

	// 发布测试消息到MQTT
	router := h.manager.GetRouter()
	err := router.PublishToMQTT(req.StationID, "test_message", req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "发布MQTT消息失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "测试消息已发布",
	})
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}
