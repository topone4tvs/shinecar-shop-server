package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "time/tzdata" // 强制设置时区数据
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
		LogDir:         "/var/www/projects/shop_server/logs",
		LogFilePrefix:  "heartbeat",
		HeartbeatMsg:   "[shop-heartbeat]: heartbeat",
		CheckInterval:  60, // 60秒检查一次
		RestartCommand: "docker compose up -d --force-recreate shop_server",
		RestartWorkDir: "/var/www",
	}
}

// HeartbeatMonitor 心跳监控器
type HeartbeatMonitor struct {
	config    *Config
	lastCheck time.Time
	lastFound time.Time
}

// NewHeartbeatMonitor 创建心跳监控器
func NewHeartbeatMonitor(cfg *Config) *HeartbeatMonitor {
	// 确保日志目录是绝对路径
	if !filepath.IsAbs(cfg.LogDir) {
		if absPath, err := filepath.Abs(cfg.LogDir); err == nil {
			cfg.LogDir = absPath
		}
	}
	// 确保重启工作目录是绝对路径
	if !filepath.IsAbs(cfg.RestartWorkDir) {
		if absPath, err := filepath.Abs(cfg.RestartWorkDir); err == nil {
			cfg.RestartWorkDir = absPath
		}
	}

	return &HeartbeatMonitor{
		config:    cfg,
		lastCheck: time.Now(),
		lastFound: time.Now(),
	}
}

// readLastLines 从文件末尾读取最后N行
func (m *HeartbeatMonitor) readLastLines(file *os.File, n int) ([]string, error) {
	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	// 如果文件为空，返回空切片
	if fileSize == 0 {
		return []string{}, nil
	}

	// 从文件末尾开始读取，读取足够的内容（假设每行平均100字节）
	// 为了安全，读取最后 10KB 的内容
	readSize := int64(10 * 1024)
	if fileSize < readSize {
		readSize = fileSize
	}

	// 定位到文件末尾
	_, err = file.Seek(-readSize, 2) // 2 表示从文件末尾
	if err != nil {
		// 如果定位失败，从头开始读取
		file.Seek(0, 0)
		readSize = fileSize
	}

	// 读取内容
	buffer := make([]byte, readSize)
	_, err = file.Read(buffer)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}

	// 将内容按行分割
	content := string(buffer)
	lines := strings.Split(content, "\n")

	// 过滤空行，并返回最后N行
	var result []string
	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			result = append([]string{line}, result...)
		}
	}

	return result, nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getLogFilePath 根据当前日期获取日志文件路径（返回绝对路径）
func (m *HeartbeatMonitor) getLogFilePath() string {
	today := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s.log", m.config.LogFilePrefix, today)
	// 使用绝对路径
	return filepath.Join(m.config.LogDir, filename)
}

// checkHeartbeat 检查日志文件中是否有心跳记录
func (m *HeartbeatMonitor) checkHeartbeat() (bool, error) {
	logFile := m.getLogFilePath()

	// 检查文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return false, fmt.Errorf("日志文件不存在: %s", logFile)
	}

	// 每次检查时重新打开文件，确保读取到最新内容
	file, err := os.Open(logFile)
	if err != nil {
		return false, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer file.Close()

	now := time.Now()

	// 从文件末尾开始，读取最后N行（默认10行），找到最新的心跳记录
	// 这样既能提高效率，又能处理最后一行不是心跳的情况（虽然理论上不会发生）
	const lastLinesToCheck = 10
	lines, err := m.readLastLines(file, lastLinesToCheck)
	if err != nil {
		return false, fmt.Errorf("读取日志文件失败: %w", err)
	}

	// Debug: 记录读取到的最后几行
	//fmt.Printf("[%s] DEBUG: 读取到 %d 行日志（从文件末尾）\n", now.Format("2006-01-02 15:04:05"), len(lines))

	var lastHeartbeatTime time.Time
	var lastHeartbeatLine string

	// 从后往前遍历（最后一行优先），找到最新的心跳记录
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// 解析 JSON 日志
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Debug: 记录解析失败的行
			fmt.Printf("[%s] DEBUG: 跳过无法解析的行: %s\n", now.Format("2006-01-02 15:04:05"), line[:min(50, len(line))])
			continue
		}

		// 检查是否是心跳消息
		if entry.Msg == m.config.HeartbeatMsg {
			// 解析时间，支持两种格式：
			// 1. 标准格式：2006-01-02 15:04:05.000
			// 2. 无毫秒格式：2006-01-02 15:04:05（兼容处理）
			// 重要：使用本地时区解析（中国标准时间），与日志记录时区保持一致
			var t time.Time
			var parseErr error
			// 先尝试带毫秒的格式，使用本地时区
			t, parseErr = time.ParseInLocation("2006-01-02 15:04:05.000", entry.Time, time.Local)
			if parseErr != nil {
				// 如果失败，尝试不带毫秒的格式
				t, parseErr = time.ParseInLocation("2006-01-02 15:04:05", entry.Time, time.Local)
			}
			if parseErr == nil {
				// Debug: 记录解析成功的心跳
				//fmt.Printf("[%s] DEBUG: 找到心跳消息，时间: %s (原始: %s), 行内容: %s\n",
				//	now.Format("2006-01-02 15:04:05"),
				//	t.Format("2006-01-02 15:04:05.000"),
				//	entry.Time,
				//	line[:min(100, len(line))])
				if t.After(lastHeartbeatTime) {
					lastHeartbeatTime = t
					lastHeartbeatLine = line
				}
			} else {
				// Debug: 记录时间解析失败
				fmt.Printf("[%s] DEBUG: 心跳消息时间解析失败: %s, 错误: %v\n",
					now.Format("2006-01-02 15:04:05"),
					entry.Time,
					parseErr)
			}
		}
	}

	// 检查最近的心跳是否在检查窗口内
	// 心跳间隔通常等于检查间隔（默认都是60秒，即1分钟一次心跳）
	// 检查逻辑：如果心跳是1分钟一次，那么每次检查时，应该能看到最近一次心跳间隔内的心跳
	// 检查窗口 = 心跳间隔 + 容错时间（30秒）
	// 如果心跳间隔是60秒，检查窗口就是 60+30=90秒
	// 这样设计的原因：
	// 1. 心跳间隔60秒，检查间隔60秒，理论上每次检查应该能看到最近60秒内的心跳
	// 2. 容错时间30秒是为了兼容：
	//    - 心跳时间戳的毫秒级偏差（如01:29:00.000和01:29:59.999，相差约60秒，但都在同一分钟内）
	//    - 检查时间的偏差（检查不是精确在整点执行，可能有几秒偏差）
	//    - 日志写入的延迟
	// 3. 实际场景验证（基于用户提供的日志）：
	//    - 最后一次心跳是01:35:00.007
	//    - 01:36:00应该有心跳（但没有）
	//    - 01:36:49检查时，距离01:35:00.007已经约109秒，超过90秒窗口，应该判定为异常
	//    - 这样就能及时检测到心跳丢失，而不是等到17分钟后
	heartbeatInterval := m.config.CheckInterval // 假设心跳间隔等于检查间隔
	checkWindow := time.Duration(heartbeatInterval+30) * time.Second

	if !lastHeartbeatTime.IsZero() {
		timeSinceLastHeartbeat := now.Sub(lastHeartbeatTime)
		// Debug: 记录时间差计算过程
		fmt.Printf("[%s] DEBUG: 时间差计算 - 当前时间: %s, 心跳时间: %s, 时间差: %.1f秒, 检查窗口: %.0f秒\n",
			now.Format("2006-01-02 15:04:05.000"),
			lastHeartbeatTime.Format("2006-01-02 15:04:05.000"),
			timeSinceLastHeartbeat.Seconds(),
			checkWindow.Seconds())
		if timeSinceLastHeartbeat <= checkWindow {
			m.lastFound = lastHeartbeatTime
			m.lastCheck = now
			// 输出调试信息，显示距离上次心跳的时间
			fmt.Printf("[%s] ✓ 心跳正常 - 最近心跳时间: %s, 距离现在: %.1f秒 (窗口: %.0f秒), 心跳行: %s\n",
				now.Format("2006-01-02 15:04:05"),
				lastHeartbeatTime.Format("2006-01-02 15:04:05.000"),
				timeSinceLastHeartbeat.Seconds(),
				checkWindow.Seconds(),
				lastHeartbeatLine[:min(100, len(lastHeartbeatLine))])
			return true, nil
		} else {
			// 心跳超时，输出详细信息
			fmt.Printf("[%s] ✗ 心跳超时 - 最近心跳时间: %s, 距离现在: %.1f秒 (超过窗口: %.0f秒), 心跳行: %s\n",
				now.Format("2006-01-02 15:04:05"),
				lastHeartbeatTime.Format("2006-01-02 15:04:05.000"),
				timeSinceLastHeartbeat.Seconds(),
				checkWindow.Seconds(),
				lastHeartbeatLine[:min(100, len(lastHeartbeatLine))])
		}
	} else {
		// Debug: 没有找到心跳消息
		fmt.Printf("[%s] DEBUG: 未找到心跳消息，读取的最后几行:\n", now.Format("2006-01-02 15:04:05"))
		for i, line := range lines {
			if len(line) > 0 {
				fmt.Printf("[%s] DEBUG:   行 %d: %s\n", now.Format("2006-01-02 15:04:05"), i+1, line[:min(100, len(line))])
			}
		}
	}

	m.lastCheck = now
	return false, nil
}

// restartDocker 执行 Docker 重启命令（使用绝对路径，不改变当前工作目录）
func (m *HeartbeatMonitor) restartDocker() error {
	fmt.Printf("[%s] 检测到心跳丢失，执行重启命令...\n", time.Now().Format("2006-01-02 15:04:05"))

	// 保存当前工作目录
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = "" // 如果获取失败，设置为空，后续不恢复
	}

	// 解析命令
	parts := strings.Fields(m.config.RestartCommand)
	if len(parts) == 0 {
		return fmt.Errorf("重启命令为空")
	}

	// 使用绝对路径设置命令工作目录，不改变当前进程的工作目录
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = m.config.RestartWorkDir

	// 执行命令
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 恢复工作目录
		if currentDir != "" {
			os.Chdir(currentDir)
		}
		return fmt.Errorf("执行重启命令失败: %w, 输出: %s", err, string(output))
	}

	// 恢复工作目录
	if currentDir != "" {
		if err := os.Chdir(currentDir); err != nil {
			fmt.Printf("[%s] ⚠ 恢复工作目录失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
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
	// 跳过 00:00 分的检查（凌晨零点那一刻）
	now := time.Now()
	if now.Hour() == 0 && now.Minute() == 0 {
		fmt.Printf("[%s] ⏭ 跳过 00:00 分的检查（避免日志文件切换时的异常）\n", now.Format("2006-01-02 15:04:05"))
		return
	}

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
		//fmt.Printf("[%s] ✓ 心跳正常\n", time.Now().Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("[%s] ✗ 心跳丢失，执行重启...\n", time.Now().Format("2006-01-02 15:04:05"))
		if err := m.restartDocker(); err != nil {
			fmt.Printf("[%s] 重启失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
	}
}

func main() {
	// 设置时区为中国标准时间（与主服务保持一致）
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 如果加载失败，使用系统本地时区
		loc = time.Local
	}
	time.Local = loc

	// 使用默认配置
	config := DefaultConfig()

	// 可以通过环境变量覆盖配置
	if logDir := os.Getenv("HEARTBEAT_LOG_DIR"); logDir != "" {
		// 转换为绝对路径
		if absPath, err := filepath.Abs(logDir); err == nil {
			config.LogDir = absPath
		} else {
			config.LogDir = logDir
		}
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
		// 转换为绝对路径
		if absPath, err := filepath.Abs(workDir); err == nil {
			config.RestartWorkDir = absPath
		} else {
			config.RestartWorkDir = workDir
		}
	}

	// 创建监控器（会再次确保路径是绝对路径）
	monitor := NewHeartbeatMonitor(config)

	// 启动监控
	if err := monitor.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "监控服务启动失败: %v\n", err)
		os.Exit(1)
	}
}
