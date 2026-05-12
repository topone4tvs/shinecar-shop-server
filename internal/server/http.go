package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"shop_server/config"
	"shop_server/internal/service"
	"shop_server/pkg/logger"

	"github.com/gin-gonic/gin"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	config  *config.Config
	server  *http.Server
	router  *gin.Engine
	manager service.ManagerInterface
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(cfg *config.Config, manager service.ManagerInterface) *HTTPServer {
	// 设置gin模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// 添加中间件
	// 自定义访问日志：仅对心跳接口 /api/device/heartbeat/:station_id 不打印 HTTP 访问日志
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		if strings.HasPrefix(param.Path, "/api/device/heartbeat/") {
			return ""
		}
		return fmt.Sprintf("%s - [%s] \"%s %s\" %d %s\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
		)
	}))
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

		// GIO推送
		api.POST("/device/gio/:station_id", h.handleGioMessage)
		api.POST("/device/serio/:station_id", h.handleSerioMessage)

		// 状态查询端点
		api.GET("/status", h.getSystemStatus)
		api.GET("/status/station/:station_id", h.getStationStatus)

		// 测试端点
		api.POST("/test/mqtt", h.testMQTTPublish)

		// 屏显配置推送
		api.POST("/display/config/:station_id", h.handleDisplayConfig)

		// 测试端点
		api.POST("/device/test/:station_id", h.handleDeviceTestOpe)
		api.POST("/device/other/test/:station_id/:ope", h.handleDeviceTestOpe)
	}

	logger.Infof("HTTP路由已设置，等待服务器初始化...")
}

// Start 启动HTTP服务器
func (h *HTTPServer) Start(ctx context.Context) error {
	logger.Infof("启动HTTP服务器: %s", h.server.Addr)

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Infof("HTTP服务器启动失败: %v", err)
		}
	}()

	logger.Infof("HTTP服务器已启动: %s", h.server.Addr)
	return nil
}

// Stop 停止HTTP服务器
func (h *HTTPServer) Stop(ctx context.Context) error {
	logger.Infof("停止HTTP服务器...")

	// 设置关闭超时
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := h.server.Shutdown(shutdownCtx); err != nil {
		logger.Infof("HTTP服务器关闭失败: %v", err)
		return err
	}

	logger.Infof("HTTP服务器已停止")
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

	logger.Infof("收到门禁设备推送: 工位=%s, IP=%s", stationID, c.ClientIP())

	// 验证工位是否存在
	if _, err := h.config.GetStationByID(stationID); err != nil {
		logger.Infof("无效的工位ID: %s", stationID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的工位ID: %s", stationID),
		})
		return
	}

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Infof("读取请求体失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取请求体失败",
		})
		return
	}

	logger.Infof("门禁推送内容: %s", string(body))

	// 解析门禁消息
	plateMsg, err := h.parsePlateMessage(body)
	if err != nil {
		logger.Infof("解析门禁消息失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析消息失败",
		})
		return
	}

	// 处理门禁消息
	response, err := h.processPlateMessage(stationID, plateMsg)
	if err != nil {
		logger.Infof("处理门禁消息失败: %v", err)
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

	// 解析 multipart/form-data
	deviceName := c.PostForm("device_name")
	ipaddr := c.PostForm("ipaddr")
	serialno := c.PostForm("serialno")
	port := c.PostForm("port")
	userName := c.PostForm("user_name")
	passWd := c.PostForm("pass_wd")
	channelNum := c.PostForm("channel_num")

	// 更新设备状态，优先用心跳体内信息
	statusData := map[string]interface{}{
		"ip_address":  ipaddr,
		"user_agent":  c.GetHeader("User-Agent"),
		"device_name": deviceName,
		"serialno":    serialno,
		"port":        port,
		"user_name":   userName,
		"pass_wd":     passWd,
		"channel_num": channelNum,
	}

	h.manager.GetDeviceManager().UpdateDeviceStatus(stationID, &service.DeviceStatus{
		DeviceType: service.DeviceTypePlate,
		StationID:  stationID,
		Online:     true,
		LastSeen:   time.Now(),
		Status:     "online",
		Data:       statusData,
	})

	// 检查是否有待处理的响应（包括MQTT指令和原有响应）
	deviceManager := h.manager.GetDeviceManager()
	if response, exists := deviceManager.GetPendingPlateResponse(stationID); exists {
		logger.Infof("心跳下发响应: 工位=%s", stationID)
		c.JSON(http.StatusOK, response)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "心跳已接收",
	})
}

// handleSnapshot 处理截图上传
func (h *HTTPServer) handleSnapshot(c *gin.Context) {
	stationID := c.Param("station_id")

	logger.Infof("收到截图上传: 工位=%s, IP=%s", stationID, c.ClientIP())

	// 读取截图数据
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Infof("读取截图数据失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取截图数据失败",
		})
		return
	}

	// 把接收到的图片数据存入本地文件中
	filePath := fmt.Sprintf("snapshot/%s.jpg", stationID)
	err = os.WriteFile(filePath, body, 0644)
	if err != nil {
		logger.Infof("保存截图数据失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存截图数据失败",
		})
		return
	}

	// 处理截图数据（这里可以保存到文件系统或云存储）
	logger.Infof("收到截图数据: 工位=%s, 大小=%d bytes", stationID, len(body))

	// 发布截图事件到MQTT（简化模式下跳过）
	if router := h.manager.GetRouter(); router != nil {
		h.publishSnapshotEvent(stationID, len(body))
	} else {
		logger.Infof("截图事件记录: 工位=%s, 大小=%d bytes (MQTT不可用)", stationID, len(body))
	}

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

	// 发布测试消息到MQTT（简化模式下跳过）
	router := h.manager.GetRouter()
	if router != nil {
		err := router.PublishToMQTT(req.StationID, "test_message", "test", req.Message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "发布MQTT消息失败",
			})
			return
		}
	} else {
		logger.Infof("测试消息记录: 工位=%s, 消息=%v (MQTT不可用)", req.StationID, req.Message)
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

func (h *HTTPServer) handleDisplayConfig(c *gin.Context) {
	stationID := c.Param("station_id")

	// log记录下完整的请求body，推送内容为json格式
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Infof("读取屏显配置推送失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取屏显配置推送失败",
		})
		return
	}

	logger.Infof("收到屏显配置推送: 工位=%s, IP=%s, 内容=%s", stationID, c.ClientIP(), string(body))

	// 解析json
	var displayConfig map[string]interface{}
	if err := json.Unmarshal(body, &displayConfig); err != nil {
		logger.Infof("解析屏显配置推送失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析屏显配置推送失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "屏显配置已接收",
	})
}

func (h *HTTPServer) handleSerioMessage(c *gin.Context) {

	stationID := c.Param("station_id")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Infof("读取GIO推送失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取GIO推送失败",
		})
		return
	}

	// 解析json
	var gioMessage map[string]interface{}
	if err := json.Unmarshal(body, &gioMessage); err != nil {
		logger.Infof("解析GIO推送失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析GIO推送失败",
		})
		return
	}

	logger.Infof("收到Serio推送: 工位=%s, IP=%s, 内容=%v", stationID, c.ClientIP(), gioMessage)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "GIO推送已接收",
	})

}

func (h *HTTPServer) handleGioMessage(c *gin.Context) {
	stationID := c.Param("station_id")

	logger.Infof("收到GIO推送: 工位=%s, IP=%s", stationID, c.ClientIP())

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Infof("读取GIO推送失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取GIO推送失败",
		})
		return
	}

	// 解析json
	var gioMessage map[string]interface{}
	if err := json.Unmarshal(body, &gioMessage); err != nil {
		logger.Infof("解析GIO推送失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析GIO推送失败",
		})
		return
	}

	logger.Infof("收到GIO推送: 工位=%s, IP=%s, 内容=%v", stationID, c.ClientIP(), gioMessage)

	// 解析门禁状态数据
	if alarmGioIn, ok := gioMessage["AlarmGioIn"].(map[string]interface{}); ok {
		if result, ok := alarmGioIn["result"].(map[string]interface{}); ok {
			if triggerResult, ok := result["TriggerResult"].(map[string]interface{}); ok {
				// 直接尝试获取 source 和 value，使用类型断言
				if source, ok := triggerResult["source"].(float64); ok {
					if value, ok := triggerResult["value"].(float64); ok {
						snapshot, changed := h.manager.GetDeviceManager().ApplyHardwareGateReading(stationID, int(source), int(value), service.GateChannelGioHTTP)

						isOpen := snapshot.IsOpen
						statusText := "开启"
						status := "open"
						if !isOpen {
							statusText = "关闭"
							status = "closed"
						}

						logger.Infof("门禁状态更新: 工位=%s, 状态=%s, 源=%d, 值=%d",
							stationID, statusText, int(source), int(value))

						if changed {
							event := MqttEvent{
								"type":       "gate_status",
								"station_id": stationID,
								"timestamp":  snapshot.LastUpdate.Unix(),
								"is_open":    isOpen,
								"status":     status,
								"source":     snapshot.Source,
								"value":      snapshot.Value,
								"channel":    snapshot.Channel,
								"raw":        gioMessage,
							}
							if err := h.publishToMQTTEvent(stationID, "gate_status", event); err != nil {
								logger.Infof("发布门禁状态事件失败: %v", err)
							}
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "GIO推送已接收",
	})
}

func (h *HTTPServer) handleDeviceTestOpe(c *gin.Context) {
	stationID := c.Param("station_id")
	ope := c.Param("ope")

	logger.Infof("收到设备测试: 工位=%s, IP=%s, 操作=%s", stationID, c.ClientIP(), ope)

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Infof("读取请求体失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取请求体失败",
		})
		return
	}

	logger.Infof("门禁推送内容: %s", string(body))

	if ope == "open" {
		cmd := &service.PlateCommand{
			BaseDeviceCommand: service.BaseDeviceCommand{
				DeviceType: service.DeviceTypePlate,
				Command:    service.CommandOpenGate,
				StationID:  stationID,
			},
		}
		// 调用开门指令
		h.manager.GetDeviceManager().ExecuteCommand(c.Request.Context(), cmd)
		logger.Infof("调用开门指令: 工位=%s, IP=%s", stationID, c.ClientIP())
	} else if ope == "close" {
		logger.Infof("收到设备测试: 工位=%s, IP=%s, 操作=关门", stationID, c.ClientIP())
	} else if ope == "display" {

	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "设备测试已接收",
	})
}
