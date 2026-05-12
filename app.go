package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"frp-client/pkg/autostart"
	"frp-client/pkg/download"
	"frp-client/pkg/frp"
	"frp-client/pkg/system"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	toolPath     string
	configPath   string
	settingsPath string
	settings     AppSettings
}

type AppSettings struct {
	ToolPath    string `json:"toolPath"`
	ConfigPath  string `json:"configPath"`
	DownloadURL string `json:"downloadUrl"`
	Theme       string `json:"theme"`
	AutoStart   bool   `json:"autoStart"`
}

type storedSettings struct {
	RootDir     string `json:"rootDir"`
	ToolsDir    string `json:"toolsDir"`
	ToolPath    string `json:"toolPath"`
	ConfigPath  string `json:"configPath"`
	DownloadURL string `json:"downloadUrl"`
	Theme       string `json:"theme"`
	AutoStart   bool   `json:"autoStart"`
}

type SettingsFileStatus struct {
	ToolExists     bool   `json:"toolExists"`
	ConfigExists   bool   `json:"configExists"`
	ToolPath       string `json:"toolPath"`
	ConfigPath     string `json:"configPath"`
	ToolHelp       string `json:"toolHelp"`
	ConfigHelp     string `json:"configHelp"`
	DownloadHelp   string `json:"downloadHelp"`
	ManualKillHelp string `json:"manualKillHelp"`
}

type DownloadTarget struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Version  string `json:"version"`
}

const defaultDownloadURLTemplate = "https://github.com/fatedier/frp/releases/download/{tag}/{filename}"

//go:embed docs/frp-help.md
var frpHelpTemplate string

func NewApp() *App {
	settingsPath := resolveSettingsPath()
	loaded := loadSettings(settingsPath)
	toolPath := normalizePath(loaded.ToolPath, defaultToolPath())
	configPath := normalizePath(loaded.ConfigPath, defaultConfigPath())
	downloadURL := normalizeDownloadURL(loaded.DownloadURL)
	theme := normalizeTheme(loaded.Theme)
	settings := AppSettings{
		ToolPath:    toolPath,
		ConfigPath:  configPath,
		DownloadURL: downloadURL,
		Theme:       theme,
		AutoStart:   autostart.IsEnabled(),
	}
	_ = ensureAppPaths(settings)
	_ = saveSettings(settingsPath, settings)
	return &App{
		toolPath:     toolPath,
		configPath:   configPath,
		settingsPath: settingsPath,
		settings:     settings,
	}
}

func resolveSettingsPath() string {
	if configDir, err := os.UserConfigDir(); err == nil {
		dir := filepath.Join(configDir, "frp-client")
		_ = os.MkdirAll(dir, 0755)
		return filepath.Join(dir, "settings.json")
	}
	return "settings.json"
}

func defaultBaseDir() string {
	if explicit := strings.TrimSpace(os.Getenv("FRP_CLIENT_HOME")); explicit != "" {
		return explicit
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, "frp-client")
	}
	return "frp-client"
}

func defaultToolPath() string {
	return filepath.Join(defaultBaseDir(), defaultToolName())
}

func defaultToolName() string {
	name := "frpc"
	if runtime.GOOS == "windows" {
		name = "frpc.exe"
	}
	return name
}

func defaultConfigPath() string {
	return filepath.Join(defaultBaseDir(), "frpc.toml")
}

func defaultDownloadURL() string {
	return defaultDownloadURLTemplate
}

func loadSettings(path string) AppSettings {
	data, err := os.ReadFile(path)
	if err != nil {
		return AppSettings{}
	}
	var stored storedSettings
	_ = json.Unmarshal(data, &stored)
	legacyBaseDir := strings.TrimSpace(stored.RootDir)
	if legacyBaseDir == "" {
		legacyBaseDir = defaultBaseDir()
	}
	if strings.TrimSpace(stored.ToolPath) == "" && (strings.TrimSpace(stored.ToolsDir) != "" || strings.TrimSpace(stored.RootDir) != "") {
		stored.ToolPath = filepath.Join(legacyBaseDir, defaultToolName())
	}
	if strings.TrimSpace(stored.ConfigPath) == "" && strings.TrimSpace(stored.RootDir) != "" {
		stored.ConfigPath = filepath.Join(legacyBaseDir, "frpc.toml")
	}
	return AppSettings{
		ToolPath:    stored.ToolPath,
		ConfigPath:  stored.ConfigPath,
		DownloadURL: stored.DownloadURL,
		Theme:       stored.Theme,
		AutoStart:   stored.AutoStart,
	}
}

func saveSettings(path string, settings AppSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建设置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化设置失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func normalizePath(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if strings.HasPrefix(value, "~") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			if value == "~" {
				value = homeDir
			} else if strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~"+string(filepath.Separator)) {
				value = filepath.Join(homeDir, strings.TrimPrefix(strings.TrimPrefix(value, "~/"), "~"+string(filepath.Separator)))
			}
		}
	}
	if abs, err := filepath.Abs(value); err == nil {
		return abs
	}
	return value
}

func normalizeTheme(theme string) string {
	theme = strings.TrimSpace(theme)
	if theme != "light" && theme != "dark" {
		return "dark"
	}
	return theme
}

func normalizeDownloadURL(downloadURL string) string {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return defaultDownloadURL()
	}
	return downloadURL
}

func ensureAppPaths(settings AppSettings) error {
	for _, dir := range []string{filepath.Dir(settings.ToolPath), filepath.Dir(settings.ConfigPath)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}
	return nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetSettings() AppSettings {
	return a.settings
}

func (a *App) SaveSettings(settings AppSettings) (AppSettings, error) {
	next := AppSettings{
		ToolPath:    normalizePath(settings.ToolPath, defaultToolPath()),
		ConfigPath:  normalizePath(settings.ConfigPath, defaultConfigPath()),
		DownloadURL: normalizeDownloadURL(settings.DownloadURL),
		Theme:       normalizeTheme(settings.Theme),
		AutoStart:   settings.AutoStart,
	}
	if runtime.GOOS == "windows" && filepath.Ext(next.ToolPath) == "" {
		next.ToolPath += ".exe"
	}
	if err := ensureAppPaths(next); err != nil {
		return a.settings, err
	}
	if next.AutoStart {
		if err := autostart.Enable(next.ToolPath, next.ConfigPath); err != nil {
			return a.settings, err
		}
	} else if err := autostart.Disable(); err != nil {
		return a.settings, err
	}
	if err := saveSettings(a.settingsPath, next); err != nil {
		return a.settings, err
	}
	a.toolPath = next.ToolPath
	a.configPath = next.ConfigPath
	a.settings = next
	return next, nil
}

func (a *App) ResetSettings() (AppSettings, error) {
	next := AppSettings{
		ToolPath:    normalizePath(defaultToolPath(), defaultToolPath()),
		ConfigPath:  normalizePath(defaultConfigPath(), defaultConfigPath()),
		DownloadURL: defaultDownloadURL(),
		Theme:       "dark",
		AutoStart:   false,
	}
	if err := ensureAppPaths(next); err != nil {
		return a.settings, err
	}
	if err := autostart.Disable(); err != nil {
		return a.settings, err
	}
	if err := saveSettings(a.settingsPath, next); err != nil {
		return a.settings, err
	}
	a.toolPath = next.ToolPath
	a.configPath = next.ConfigPath
	a.settings = next
	return next, nil
}

func (a *App) ChooseFile(title string) (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: title,
	})
}

func (a *App) CheckSettingsFiles() SettingsFileStatus {
	status := SettingsFileStatus{
		ToolPath:       a.toolPath,
		ConfigPath:     a.configPath,
		ToolHelp:       toolHelp(a.toolPath),
		ConfigHelp:     configHelp(a.configPath),
		DownloadHelp:   downloadHelp(filepath.Dir(a.toolPath)),
		ManualKillHelp: manualKillHelp(),
	}
	if _, err := os.Stat(a.toolPath); err == nil {
		status.ToolExists = true
	}
	if _, err := os.Stat(a.configPath); err == nil {
		status.ConfigExists = true
	}
	return status
}

func toolHelp(toolPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Windows: 请手动下载 frp windows_amd64.zip，并把 frpc.exe 解压到工具路径: %s。设置里的工具路径必须和 frpc.exe 实际位置一致。", toolPath)
	}
	return fmt.Sprintf("macOS: 请手动下载 frp darwin 包，并把 frpc 解压到工具路径: %s。设置里的工具路径必须和 frpc 实际位置一致。", toolPath)
}

func configHelp(configPath string) string {
	return fmt.Sprintf("配置文件不存在时，请先创建 frpc.toml 或在编辑器中保存配置到: %s", configPath)
}

func downloadHelp(toolDir string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Windows: 内置下载会把 frpc.exe 解压到 %s。若安全软件拦截，请手动下载并解压到同一目录。", toolDir)
	}
	return fmt.Sprintf("macOS: 内置下载会把 frpc 解压到 %s。手动下载时也请解压到同一目录。", toolDir)
}

func manualKillHelp() string {
	if runtime.GOOS == "windows" {
		return "Windows: 如自动结束失败，请在终端执行 taskkill /PID <pid> /F。"
	}
	return "macOS: 如自动结束失败，请在终端执行 kill -9 <pid>。"
}

// ========== 系统信息 ==========

func (a *App) GetSystemInfo() system.SystemInfo {
	return system.GetSystemInfo()
}

// ========== 镜像源 ==========

func (a *App) GetMirrors() ([]download.Mirror, error) {
	mirrorPath := filepath.Join(filepath.Dir(a.configPath), "mirrors.yaml")
	if _, err := os.Stat(mirrorPath); os.IsNotExist(err) {
		return []download.Mirror{
			{Name: "GitHub 官方", Type: "github", Template: "https://github.com/fatedier/frp/releases/download/{tag}/{filename}"},
			{Name: "ghproxy.com", Type: "proxy", Template: "https://ghproxy.com/https://github.com/fatedier/frp/releases/download/{tag}/{filename}"},
		}, nil
	}
	return download.LoadMirrors(mirrorPath)
}

// ========== frp 版本 ==========

func (a *App) GetFrpVersions() ([]string, error) {
	versions, err := download.FetchFrpVersions()
	if err != nil {
		return []string{"v0.68.0", "v0.67.0", "v0.66.0"}, nil
	}
	return versions, nil
}

func (a *App) GetDownloadTarget(version string, osName string, arch string) DownloadTarget {
	return a.buildDownloadTarget(version, osName, arch)
}

func (a *App) buildDownloadTarget(version string, osName string, arch string) DownloadTarget {
	if osName == "" || arch == "" {
		info := system.GetSystemInfo()
		if osName == "" {
			osName = info.OS
		}
		if arch == "" {
			arch = info.Arch
		}
	}
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}
	filename := download.BuildFilename(version, osName, arch)
	url := download.BuildDownloadURLFromTemplate(a.settings.DownloadURL, version, filename)
	return DownloadTarget{
		URL:      url,
		Filename: filename,
		Version:  version,
	}
}

func (a *App) GetFrpHelp(version string, osName string, arch string) string {
	target := a.buildDownloadTarget(version, osName, arch)
	replacements := map[string]string{
		"{{DOWNLOAD_URL}}": target.URL,
		"{{FILENAME}}":     target.Filename,
		"{{TOOL_PATH}}":    a.toolPath,
		"{{CONFIG_PATH}}":  a.configPath,
		"{{WORK_DIR}}":     defaultBaseDir(),
	}
	content := frpHelpTemplate
	for from, to := range replacements {
		content = strings.ReplaceAll(content, from, to)
	}
	return content
}

// ========== frpc 安装检测 ==========

func (a *App) CheckFrpcInstalled() bool {
	_, err := os.Stat(a.toolPath)
	return err == nil
}

func (a *App) GetFrpcVersion() string {
	return download.ReadVersionFile(filepath.Dir(a.toolPath))
}

// ========== 下载 frpc ==========

func (a *App) DownloadFrpc(version string, mirrorName string, osName string, arch string) error {
	if osName == "" {
		info := system.GetSystemInfo()
		osName = info.OS
		arch = info.Arch
	}

	target := a.buildDownloadTarget(version, osName, arch)
	url := target.URL
	toolDir := filepath.Dir(a.toolPath)
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return fmt.Errorf("创建工具路径目录失败: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "frpc-download")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	ctx := download.StartDownloadContext()
	defer download.FinishDownloadContext()

	filename := target.Filename
	archivePath := filepath.Join(tmpDir, filename)

	progress := download.GetDownloadProgress()
	progress.Reset()

	if err := download.DownloadFile(ctx, url, archivePath, progress); err != nil {
		progress.SetError(err.Error())
		if err == download.ErrDownloadCanceled {
			_ = os.Remove(archivePath)
			progress.SetError("下载已停止")
			return fmt.Errorf("下载已停止")
		}
		return fmt.Errorf("下载失败: %w", err)
	}

	if err := download.ExtractFrpcToFile(archivePath, a.toolPath, osName); err != nil {
		progress.SetError(err.Error())
		if isWindowsSecurityBlockError(err) {
			return fmt.Errorf("Windows 安全中心或杀毒软件拦截了 frpc 压缩包。请手动下载 %s，解压其中的 frpc.exe 到工具路径所在目录：%s。注意：设置里的工具路径必须和解压后的 frpc.exe 路径一致", filename, toolDir)
		}
		return fmt.Errorf("解压失败: %w", err)
	}

	if err := download.WriteVersionFile(toolDir, version); err != nil {
		return fmt.Errorf("写入版本文件失败: %w", err)
	}

	progress.SetDone(true)
	return nil
}

func (a *App) CancelFrpcDownload() {
	download.CancelDownload()
}

func isWindowsSecurityBlockError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "virus") ||
		strings.Contains(message, "potentially unwanted software") ||
		strings.Contains(message, "operation did not complete successfully")
}

func (a *App) GetDownloadProgress() float64 {
	progress := download.GetDownloadProgress()
	return progress.GetPercentage()
}

// ========== 配置管理 ==========

func (a *App) ReadConfig() (string, error) {
	content, err := frp.ReadConfig(a.configPath)
	if err == nil {
		return content, nil
	}
	if os.IsNotExist(err) {
		return frp.DefaultConfig(), nil
	}
	return "", err
}

func (a *App) WriteConfig(content string) error {
	return frp.WriteConfig(a.configPath, content)
}

func (a *App) ValidateConfig() (string, error) {
	return frp.ValidateConfig(a.configPath, a.toolPath)
}

// ========== 进程管理 ==========

func (a *App) GetFrpStatus() frp.FrpStatus {
	return frp.GetStatus(a.configPath, a.toolPath)
}

func (a *App) ListFrpcProcesses() (frp.FrpcProcessInfo, error) {
	return frp.ListFrpcProcesses()
}

func (a *App) KillFrpcProcesses(pids []int) error {
	return frp.KillFrpcProcesses(pids)
}

func (a *App) StartFrp() error {
	return frp.StartFrp(a.configPath, a.toolPath)
}

func (a *App) StopFrp() error {
	return frp.StopFrp(a.configPath)
}

func (a *App) RestartFrp() error {
	return frp.RestartFrp(a.configPath, a.toolPath)
}

func (a *App) GetFrpLogs(lines int) string {
	return frp.GetLogs(a.configPath, lines)
}
