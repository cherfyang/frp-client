package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"frp-client/pkg/download"
	"frp-client/pkg/frp"
	"frp-client/pkg/system"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	rootDir      string
	toolsDir     string
	settingsPath string
	settings     AppSettings
}

type AppSettings struct {
	RootDir  string `json:"rootDir"`
	ToolsDir string `json:"toolsDir"`
	Theme    string `json:"theme"`
}

func NewApp() *App {
	settingsPath := resolveSettingsPath()
	settings := loadSettings(settingsPath)
	rootDir := normalizeDir(settings.RootDir, defaultRootDir())
	toolsDir := normalizeDir(settings.ToolsDir, filepath.Join(rootDir, ".tools"))
	theme := strings.TrimSpace(settings.Theme)
	if theme != "light" && theme != "dark" {
		theme = "dark"
	}
	settings = AppSettings{
		RootDir:  rootDir,
		ToolsDir: toolsDir,
		Theme:    theme,
	}
	_ = ensureAppDirs(settings)
	_ = saveSettings(settingsPath, settings)
	return &App{
		rootDir:      rootDir,
		toolsDir:     toolsDir,
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

func defaultRootDir() string {
	if explicit := strings.TrimSpace(os.Getenv("FRP_CLIENT_HOME")); explicit != "" {
		return explicit
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, "frp-client")
	}
	return "frp-client"
}

func loadSettings(path string) AppSettings {
	data, err := os.ReadFile(path)
	if err != nil {
		return AppSettings{}
	}
	var settings AppSettings
	_ = json.Unmarshal(data, &settings)
	return settings
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

func normalizeDir(value, fallback string) string {
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

func ensureAppDirs(settings AppSettings) error {
	if err := os.MkdirAll(settings.RootDir, 0755); err != nil {
		return fmt.Errorf("创建工作目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(settings.RootDir, "config"), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.MkdirAll(settings.ToolsDir, 0755); err != nil {
		return fmt.Errorf("创建工具目录失败: %w", err)
	}
	return nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetRootDir() string {
	return a.rootDir
}

func (a *App) GetSettings() AppSettings {
	return a.settings
}

func (a *App) SaveSettings(settings AppSettings) (AppSettings, error) {
	rootDir := normalizeDir(settings.RootDir, defaultRootDir())
	toolsDir := normalizeDir(settings.ToolsDir, filepath.Join(rootDir, ".tools"))
	theme := strings.TrimSpace(settings.Theme)
	if theme != "light" && theme != "dark" {
		theme = "dark"
	}
	next := AppSettings{
		RootDir:  rootDir,
		ToolsDir: toolsDir,
		Theme:    theme,
	}
	if err := ensureAppDirs(next); err != nil {
		return a.settings, err
	}
	if err := saveSettings(a.settingsPath, next); err != nil {
		return a.settings, err
	}
	a.rootDir = next.RootDir
	a.toolsDir = next.ToolsDir
	a.settings = next
	return next, nil
}

func (a *App) ChooseDirectory(title string) (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: title,
	})
}

// ========== 系统信息 ==========

func (a *App) GetSystemInfo() system.SystemInfo {
	return system.GetSystemInfo()
}

// ========== 镜像源 ==========

func (a *App) GetMirrors() ([]download.Mirror, error) {
	mirrorPath := filepath.Join(a.rootDir, "mirrors.yaml")
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

// ========== frpc 安装检测 ==========

func (a *App) CheckFrpcInstalled() bool {
	platform := system.GetPlatformDir()
	return download.CheckFrpcExists(a.toolsDir, platform)
}

func (a *App) GetFrpcVersion() string {
	return download.ReadVersionFile(a.toolsDir)
}

// ========== 下载 frpc ==========

func (a *App) DownloadFrpc(version string, mirrorName string, osName string, arch string) error {
	if osName == "" {
		info := system.GetSystemInfo()
		osName = info.OS
		arch = info.Arch
	}

	mirrors, err := a.GetMirrors()
	if err != nil {
		return fmt.Errorf("读取镜像源失败: %w", err)
	}

	var selectedMirror download.Mirror
	for _, m := range mirrors {
		if m.Name == mirrorName {
			selectedMirror = m
			break
		}
	}
	if selectedMirror.Name == "" && len(mirrors) > 0 {
		selectedMirror = mirrors[0]
	}
	if selectedMirror.Name == "" {
		return fmt.Errorf("没有可用的下载镜像源")
	}

	url := download.BuildDownloadURL(selectedMirror, version, osName, arch)

	platformDir := filepath.Join(a.toolsDir, osName)
	os.MkdirAll(platformDir, 0755)

	tmpDir, err := os.MkdirTemp("", "frpc-download")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	filename := download.BuildFilename(version, osName, arch)
	archivePath := filepath.Join(tmpDir, filename)

	progress := download.GetDownloadProgress()
	progress.Reset()

	if err := download.DownloadFile(url, archivePath, progress); err != nil {
		progress.SetError(err.Error())
		return fmt.Errorf("下载失败: %w", err)
	}

	if err := download.ExtractFrpc(archivePath, platformDir, osName); err != nil {
		progress.SetError(err.Error())
		return fmt.Errorf("解压失败: %w", err)
	}

	if err := download.WriteVersionFile(a.toolsDir, version); err != nil {
		return fmt.Errorf("写入版本文件失败: %w", err)
	}

	progress.SetDone(true)
	return nil
}

func (a *App) GetDownloadProgress() float64 {
	progress := download.GetDownloadProgress()
	return progress.GetPercentage()
}

// ========== 配置管理 ==========

func (a *App) ReadConfig() (string, error) {
	return frp.ReadConfig(a.rootDir)
}

func (a *App) WriteConfig(content string) error {
	return frp.WriteConfig(a.rootDir, content)
}

func (a *App) ValidateConfig() (string, error) {
	return frp.ValidateConfig(a.rootDir, a.toolsDir)
}

// ========== 进程管理 ==========

func (a *App) GetFrpStatus() frp.FrpStatus {
	return frp.GetStatus(a.rootDir, a.toolsDir)
}

func (a *App) StartFrp() error {
	return frp.StartFrp(a.rootDir, a.toolsDir)
}

func (a *App) StopFrp() error {
	return frp.StopFrp(a.rootDir)
}

func (a *App) RestartFrp() error {
	return frp.RestartFrp(a.rootDir, a.toolsDir)
}

func (a *App) GetFrpLogs(lines int) string {
	return frp.GetLogs(a.rootDir, lines)
}
