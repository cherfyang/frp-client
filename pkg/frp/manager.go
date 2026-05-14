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

type FrpcProcessInfo struct {
	PIDs        []int  `json:"pids"`
	KillCommand string `json:"killCommand"`
	Message     string `json:"message"`
}

func ConfigBaseDir(configPath string) string {
	dir := filepath.Dir(configPath)
	if dir == "." || dir == "" {
		return "."
	}
	return dir
}

func GetPidPath(configPath string) string {
	return filepath.Join(ConfigBaseDir(configPath), ".frpc.pid")
}

func GetLogPath(configPath string) string {
	return filepath.Join(ConfigBaseDir(configPath), ".frpc.log")
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

func ListFrpcProcesses() (FrpcProcessInfo, error) {
	pids, err := listSystemFrpcPIDs()
	if err != nil {
		return FrpcProcessInfo{}, fmt.Errorf("检查本机 frpc 进程失败: %w", err)
	}
	return FrpcProcessInfo{
		PIDs:        pids,
		KillCommand: BuildKillCommand(pids),
		Message:     BuildProcessMessage(pids),
	}, nil
}

func KillFrpcProcesses(pids []int) error {
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
	if len(failures) > 0 {
		return fmt.Errorf("停止已运行的 frpc 失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

func BuildKillCommand(pids []int) string {
	if len(pids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(pids))
	for _, pid := range pids {
		if runtime.GOOS == "windows" {
			parts = append(parts, fmt.Sprintf("taskkill /PID %d /F", pid))
		} else {
			parts = append(parts, fmt.Sprintf("kill -9 %d", pid))
		}
	}
	return strings.Join(parts, " && ")
}

func BuildProcessMessage(pids []int) string {
	if len(pids) == 0 {
		return ""
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("检测到本机已有 frpc.exe 进程正在运行，PID: %s。是否先结束这些进程？", joinPIDs(pids))
	}
	return fmt.Sprintf("检测到本机已有 frpc 进程正在运行，PID: %s。是否先结束这些进程？", joinPIDs(pids))
}

func joinPIDs(pids []int) string {
	parts := make([]string, 0, len(pids))
	for _, pid := range pids {
		parts = append(parts, strconv.Itoa(pid))
	}
	return strings.Join(parts, ", ")
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

func GetStatus(configPath, toolPath string) FrpStatus {
	status := FrpStatus{
		Running:    false,
		LogPath:    GetLogPath(configPath),
		ConfigPath: configPath,
		BinaryPath: toolPath,
	}

	pidFile := GetPidPath(configPath)
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

	if toolPath != "" {
		cmd := exec.Command(toolPath, "-v")
		output, _ := cmd.CombinedOutput()
		status.Version = strings.TrimSpace(string(output))
	}

	return status
}

func StartFrp(configPath, toolPath string) error {
	if _, err := os.Stat(toolPath); err != nil {
		return fmt.Errorf("未找到 frpc 可执行文件: %s", toolPath)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", configPath)
	}

	baseDir := ConfigBaseDir(configPath)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	pidFile := GetPidPath(configPath)
	logFile := GetLogPath(configPath)

	logWriter, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer logWriter.Close()

	cmd := exec.Command(toolPath, "-c", configPath)
	cmd.Dir = baseDir
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

func StopFrp(configPath string) error {
	pidFile := GetPidPath(configPath)
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

func RestartFrp(configPath, toolPath string) error {
	return StartFrp(configPath, toolPath)
}

func GetLogs(configPath string, lines int) string {
	logPath := GetLogPath(configPath)
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
