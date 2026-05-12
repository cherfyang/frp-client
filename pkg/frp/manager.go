package frp

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type FrpStatus struct {
	Running    bool   `json:"running"`
	PID        int    `json:"pid"`
	Uptime     string `json:"uptime"`
	Version    string `json:"version"`
	LogPath    string `json:"logPath"`
	ConfigPath string `json:"configPath"`
	BinaryPath string `json:"binaryPath"`
}

func FindManagedFrpcBinary(toolsDir string) string {
	var platform string
	switch runtime.GOOS {
	case "darwin":
		platform = "darwin"
	case "windows":
		platform = "windows"
	default:
		platform = "linux"
	}

	ext := ""
	if platform == "windows" {
		ext = ".exe"
	}
	frpcPath := filepath.Join(toolsDir, platform, "frpc"+ext)

	if _, err := os.Stat(frpcPath); err == nil {
		return frpcPath
	}
	return ""
}

func FindFrpcBinary(toolsDir string) string {
	if frpcPath := FindManagedFrpcBinary(toolsDir); frpcPath != "" {
		return frpcPath
	}

	pathFrpc := "frpc"
	if runtime.GOOS == "windows" {
		pathFrpc = "frpc.exe"
	}
	if _, err := exec.LookPath(pathFrpc); err == nil {
		return pathFrpc
	}

	return ""
}

func GetPidPath(rootDir string) string {
	return filepath.Join(rootDir, ".frpc.pid")
}

func GetLogPath(rootDir string) string {
	return filepath.Join(rootDir, ".frpc.log")
}

func readPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
		output, err := cmd.Output()
		return err == nil && strings.Contains(strings.ToLower(string(output)), "frpc")
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func listSystemFrpcPIDs() ([]int, error) {
	if runtime.GOOS == "windows" {
		return listWindowsFrpcPIDs()
	}
	pids, err := listUnixFrpcPIDsByPgrep()
	if err == nil {
		return pids, nil
	}
	return listUnixFrpcPIDsByPS()
}

func listUnixFrpcPIDsByPgrep() ([]int, error) {
	output, err := exec.Command("pgrep", "-x", "frpc").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(output) == 0 && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	return parsePIDLines(string(output)), nil
}

func listUnixFrpcPIDsByPS() ([]int, error) {
	output, err := exec.Command("ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		if filepath.Base(fields[1]) != "frpc" {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func listWindowsFrpcPIDs() ([]int, error) {
	output, err := exec.Command("tasklist", "/FI", "IMAGENAME eq frpc.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, record := range records {
		if len(record) < 2 || !strings.EqualFold(record[0], "frpc.exe") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(record[1]))
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func parsePIDLines(output string) []int {
	var pids []int
	for _, line := range strings.Split(output, "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func stopProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 20; i++ {
		if !isProcessRunning(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return process.Signal(syscall.SIGKILL)
}

func StopSystemFrpcProcesses(rootDir string) error {
	pids, err := listSystemFrpcPIDs()
	if err != nil {
		return fmt.Errorf("检查本机 frpc 进程失败: %w", err)
	}
	if len(pids) == 0 {
		return nil
	}

	var failures []string
	currentPID := os.Getpid()
	for _, pid := range pids {
		if pid <= 0 || pid == currentPID {
			continue
		}
		if err := stopProcess(pid); err != nil {
			failures = append(failures, fmt.Sprintf("PID %d: %v", pid, err))
		}
	}
	_ = os.Remove(GetPidPath(rootDir))
	if len(failures) > 0 {
		return fmt.Errorf("停止已运行的 frpc 失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

func readProcessStartTime(pid int) (time.Time, error) {
	if runtime.GOOS == "windows" {
		return time.Now(), nil
	}
	if runtime.GOOS == "darwin" {
		output, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			return time.Time{}, err
		}
		value := strings.TrimSpace(string(output))
		if value == "" {
			return time.Time{}, fmt.Errorf("无法读取进程启动时间")
		}
		startTime, err := time.Parse("Mon Jan 2 15:04:05 2006", value)
		if err != nil {
			return time.Time{}, err
		}
		return startTime, nil
	}
	pidStr := strconv.Itoa(pid)
	data, err := os.ReadFile(fmt.Sprintf("/proc/%s/stat", pidStr))
	if err != nil {
		return time.Time{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 22 {
		return time.Time{}, fmt.Errorf("无法解析进程启动时间")
	}
	starttime := fields[21]
	ticks, err := strconv.ParseUint(starttime, 10, 64)
	if err != nil {
		return time.Now(), nil
	}
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return time.Now(), nil
	}
	uptimeFields := strings.Fields(string(uptimeData))
	if len(uptimeFields) < 1 {
		return time.Now(), nil
	}
	uptimeSeconds, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return time.Now(), nil
	}
	clkTck := float64(100)
	bootTime := time.Now().Add(-time.Duration(uptimeSeconds * float64(time.Second)))
	startTime := bootTime.Add(time.Duration(float64(ticks) / clkTck * float64(time.Second)))
	return startTime, nil
}

func GetStatus(rootDir, toolsDir string) FrpStatus {
	status := FrpStatus{
		Running:    false,
		LogPath:    GetLogPath(rootDir),
		ConfigPath: GetConfigPath(rootDir),
	}

	pidFile := GetPidPath(rootDir)
	pid, err := readPID(pidFile)
	if err != nil {
		return status
	}

	if !isProcessRunning(pid) {
		os.Remove(pidFile)
		return status
	}

	status.PID = pid
	status.Running = true

	startTime, err := readProcessStartTime(pid)
	if err == nil {
		duration := time.Since(startTime)
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		seconds := int(duration.Seconds()) % 60
		if hours > 0 {
			status.Uptime = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
		} else if minutes > 0 {
			status.Uptime = fmt.Sprintf("%dm %ds", minutes, seconds)
		} else {
			status.Uptime = fmt.Sprintf("%ds", seconds)
		}
	}

	frpcPath := FindFrpcBinary(toolsDir)
	if frpcPath != "" {
		status.BinaryPath = frpcPath
		cmd := exec.Command(frpcPath, "--version")
		output, _ := cmd.CombinedOutput()
		status.Version = strings.TrimSpace(string(output))
	}

	return status
}

func StartFrp(rootDir, toolsDir string) error {
	frpcPath := FindFrpcBinary(toolsDir)
	if frpcPath == "" {
		return fmt.Errorf("未找到 frpc 可执行文件，请先安装")
	}

	configPath := GetConfigPath(rootDir)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", configPath)
	}

	if err := StopSystemFrpcProcesses(rootDir); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)

	pidFile := GetPidPath(rootDir)
	logFile := GetLogPath(rootDir)

	logWriter, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer logWriter.Close()

	cmd := exec.Command(frpcPath, "-c", configPath)
	cmd.Dir = rootDir
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 frpc 失败: %w", err)
	}

	pidData := fmt.Sprintf("%d\n", cmd.Process.Pid)
	if err := os.WriteFile(pidFile, []byte(pidData), 0644); err != nil {
		return fmt.Errorf("写入 PID 文件失败: %w", err)
	}

	time.Sleep(1 * time.Second)

	if !isProcessRunning(cmd.Process.Pid) {
		os.Remove(pidFile)
		logContent, _ := os.ReadFile(logFile)
		logSnippet := string(logContent)
		if len(logSnippet) > 500 {
			logSnippet = logSnippet[len(logSnippet)-500:]
		}
		return fmt.Errorf("frpc 启动后立即退出，日志: %s", logSnippet)
	}

	go func() {
		cmd.Wait()
		os.Remove(pidFile)
	}()

	return nil
}

func StopFrp(rootDir string) error {
	pidFile := GetPidPath(rootDir)
	pid, err := readPID(pidFile)
	if err != nil {
		return fmt.Errorf("frpc 未在运行")
	}

	if !isProcessRunning(pid) {
		os.Remove(pidFile)
		return fmt.Errorf("frpc 进程已不存在")
	}

	if err := stopProcess(pid); err != nil {
		return fmt.Errorf("停止 frpc 失败: %w", err)
	}

	os.Remove(pidFile)
	return nil
}

func RestartFrp(rootDir, toolsDir string) error {
	return StartFrp(rootDir, toolsDir)
}

func GetLogs(rootDir string, lines int) string {
	logPath := GetLogPath(rootDir)
	data, err := readTail(logPath, 512*1024)
	if err != nil {
		return ""
	}

	allLines := strings.Split(string(data), "\n")
	var filtered []string
	for _, line := range allLines {
		if line != "" {
			filtered = append(filtered, line)
		}
	}

	if lines > 0 && len(filtered) > lines {
		filtered = filtered[len(filtered)-lines:]
	}

	return strings.Join(filtered, "\n")
}

func readTail(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	if size <= maxBytes {
		return io.ReadAll(file)
	}

	if _, err := file.Seek(-maxBytes, io.SeekEnd); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if index := bytes.IndexByte(data, '\n'); index >= 0 && index+1 < len(data) {
		return data[index+1:], nil
	}
	return data, nil
}
