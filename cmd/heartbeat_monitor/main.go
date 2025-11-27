package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LogEntry 日志条目结构
type LogEntry struct {
	Level  string `json:"level"`
	Time   string `json:"time"`
	Caller string `json:"caller"`
	Msg    string `json:"msg"`
}

// Config 监控配置
type Config struct {
	// 日志文件目录
	LogDir string `json:"log_dir"`
	// 日志文件名前缀（不含日期部分）
	LogFilePrefix string `json:"log_file_prefix"`
	// 心跳消息内容
	HeartbeatMsg string `json:"heartbeat_msg"`
	// 检查间隔（秒）
	CheckInterval int `json:"check_interval"`
	// Docker 重启命令
	RestartCommand string `json:"restart_command"`
	// 重启命令工作目录
	RestartWorkDir string `json:"restart_work_dir"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		LogDir:         "logs",
		LogFilePrefix:  "heartbeat",
		HeartbeatMsg:   "[shop-heartbeat]: heartbeat",
		CheckInterval:  60, // 60秒检查一次
		RestartCommand: "docker compose up -d --force-recreate shop_server",
		RestartWorkDir: "/var/www",
	}
}

// HeartbeatMonitor 心跳监控器
type HeartbeatMonitor struct {
	config     *Config
	lastCheck  time.Time
	lastFound  time.Time
}

// NewHeartbeatMonitor 创建心跳监控器
func NewHeartbeatMonitor(cfg *Config) *HeartbeatMonitor {
	return &HeartbeatMonitor{
		config:    cfg,
		lastCheck: time.Now(),
		lastFound: time.Now(),
	}
}

// getLogFilePath 根据当前日期获取日志文件路径
func (m *HeartbeatMonitor) getLogFilePath() string {
	today := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s.log", m.config.LogFilePrefix, today)
	return filepath.Join(m.config.LogDir, filename)
}

// checkHeartbeat 检查日志文件中是否有心跳记录
func (m *HeartbeatMonitor) checkHeartbeat() (bool, error) {
	logFile := m.getLogFilePath()
	
	// 检查文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return false, fmt.Errorf("日志文件不存在: %s", logFile)
	}

	// 打开文件
	file, err := os.Open(logFile)
	if err != nil {
		return false, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer file.Close()

	now := time.Now()
	
	// 读取文件，查找最近的心跳记录
	scanner := bufio.NewScanner(file)
	var lastHeartbeatTime time.Time

	// 遍历所有行，找到最新的心跳记录
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 解析 JSON 日志
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // 跳过无法解析的行
		}

		// 检查是否是心跳消息
		if entry.Msg == m.config.HeartbeatMsg {
			// 解析时间
			if t, err := time.Parse("2006-01-02 15:04:05.000", entry.Time); err == nil {
				if t.After(lastHeartbeatTime) {
					lastHeartbeatTime = t
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("读取日志文件失败: %w", err)
	}

	// 检查最近的心跳是否在检查窗口内
	// 检查窗口 = 检查间隔 + 容错时间（30秒），确保能检测到每分钟的心跳
	checkWindow := time.Duration(m.config.CheckInterval+30) * time.Second
	if !lastHeartbeatTime.IsZero() {
		timeSinceLastHeartbeat := now.Sub(lastHeartbeatTime)
		if timeSinceLastHeartbeat <= checkWindow {
			m.lastFound = lastHeartbeatTime
			m.lastCheck = now
			return true, nil
		}
	}

	m.lastCheck = now
	return false, nil
}

// restartDocker 执行 Docker 重启命令
func (m *HeartbeatMonitor) restartDocker() error {
	fmt.Printf("[%s] 检测到心跳丢失，执行重启命令...\n", time.Now().Format("2006-01-02 15:04:05"))
	
	// 切换到工作目录
	if err := os.Chdir(m.config.RestartWorkDir); err != nil {
		return fmt.Errorf("切换工作目录失败: %w", err)
	}

	// 解析命令
	parts := strings.Fields(m.config.RestartCommand)
	if len(parts) == 0 {
		return fmt.Errorf("重启命令为空")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = m.config.RestartWorkDir
	
	// 执行命令
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行重启命令失败: %w, 输出: %s", err, string(output))
	}

	fmt.Printf("[%s] 重启命令执行成功，输出: %s\n", time.Now().Format("2006-01-02 15:04:05"), string(output))
	return nil
}

// Start 启动监控
func (m *HeartbeatMonitor) Start() error {
	fmt.Printf("[%s] 启动心跳监控服务...\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("  日志目录: %s\n", m.config.LogDir)
	fmt.Printf("  日志文件前缀: %s\n", m.config.LogFilePrefix)
	fmt.Printf("  心跳消息: %s\n", m.config.HeartbeatMsg)
	fmt.Printf("  检查间隔: %d 秒\n", m.config.CheckInterval)
	fmt.Printf("  重启命令: %s\n", m.config.RestartCommand)
	fmt.Printf("  工作目录: %s\n", m.config.RestartWorkDir)

	// 创建定时器
	ticker := time.NewTicker(time.Duration(m.config.CheckInterval) * time.Second)
	defer ticker.Stop()

	// 立即执行一次检查
	m.checkAndRestart()

	// 定时检查
	for range ticker.C {
		m.checkAndRestart()
	}

	return nil
}

// checkAndRestart 检查心跳并执行重启（如果需要）
func (m *HeartbeatMonitor) checkAndRestart() {
	found, err := m.checkHeartbeat()
	if err != nil {
		fmt.Printf("[%s] ⚠ 检查心跳失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		// 如果是文件不存在，可能是服务刚启动，先不重启
		if strings.Contains(err.Error(), "日志文件不存在") {
			fmt.Printf("[%s] 日志文件不存在，等待文件创建...\n", time.Now().Format("2006-01-02 15:04:05"))
			return
		}
		// 其他错误时执行重启
		fmt.Printf("[%s] ✗ 心跳检查异常，执行重启...\n", time.Now().Format("2006-01-02 15:04:05"))
		if err := m.restartDocker(); err != nil {
			fmt.Printf("[%s] 重启失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
		return
	}

	if found {
		fmt.Printf("[%s] ✓ 心跳正常\n", time.Now().Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("[%s] ✗ 心跳丢失，执行重启...\n", time.Now().Format("2006-01-02 15:04:05"))
		if err := m.restartDocker(); err != nil {
			fmt.Printf("[%s] 重启失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
	}
}

func main() {
	// 使用默认配置
	config := DefaultConfig()

	// 可以通过环境变量覆盖配置
	if logDir := os.Getenv("HEARTBEAT_LOG_DIR"); logDir != "" {
		config.LogDir = logDir
	}
	if logPrefix := os.Getenv("HEARTBEAT_LOG_PREFIX"); logPrefix != "" {
		config.LogFilePrefix = logPrefix
	}
	if heartbeatMsg := os.Getenv("HEARTBEAT_MSG"); heartbeatMsg != "" {
		config.HeartbeatMsg = heartbeatMsg
	}
	if checkInterval := os.Getenv("HEARTBEAT_CHECK_INTERVAL"); checkInterval != "" {
		if interval, err := time.ParseDuration(checkInterval); err == nil {
			config.CheckInterval = int(interval.Seconds())
		}
	}
	if restartCmd := os.Getenv("HEARTBEAT_RESTART_CMD"); restartCmd != "" {
		config.RestartCommand = restartCmd
	}
	if workDir := os.Getenv("HEARTBEAT_WORK_DIR"); workDir != "" {
		config.RestartWorkDir = workDir
	}

	// 创建监控器
	monitor := NewHeartbeatMonitor(config)

	// 启动监控
	if err := monitor.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "监控服务启动失败: %v\n", err)
		os.Exit(1)
	}
}

