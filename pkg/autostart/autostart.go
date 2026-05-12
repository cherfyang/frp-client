package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"frp-client/pkg/frp"
)

const (
	launchAgentLabel = "com.frpclient.frpc"
	desktopFileName  = "frp-client-frpc.desktop"
	windowsRunName   = "frp-client-frpc"
)

func Enable(rootDir, toolsDir string) error {
	frpcPath := frp.FindManagedFrpcBinary(toolsDir)
	if frpcPath == "" {
		return fmt.Errorf("工具目录中未找到 frpc 可执行文件，请先安装")
	}
	configPath := frp.GetConfigPath(rootDir)
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("配置文件不存在: %s", configPath)
	}

	switch runtime.GOOS {
	case "darwin":
		return enableLaunchAgent(rootDir, frpcPath, configPath)
	case "windows":
		return enableWindowsRun(rootDir, frpcPath, configPath)
	case "linux":
		return enableLinuxAutostart(rootDir, frpcPath, configPath)
	default:
		return fmt.Errorf("当前系统暂不支持开机自启动: %s", runtime.GOOS)
	}
}

func Disable() error {
	switch runtime.GOOS {
	case "darwin":
		return removeIfExists(launchAgentPath())
	case "windows":
		if !IsEnabled() {
			return nil
		}
		return exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", windowsRunName, "/f").Run()
	case "linux":
		return removeIfExists(linuxDesktopPath())
	default:
		return fmt.Errorf("当前系统暂不支持开机自启动: %s", runtime.GOOS)
	}
}

func IsEnabled() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat(launchAgentPath())
		return err == nil
	case "windows":
		err := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", windowsRunName).Run()
		return err == nil
	case "linux":
		_, err := os.Stat(linuxDesktopPath())
		return err == nil
	default:
		return false
	}
}

func enableLaunchAgent(rootDir, frpcPath, configPath string) error {
	path := launchAgentPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建 LaunchAgents 目录失败: %w", err)
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>-c</string>
    <string>%s</string>
  </array>
  <key>WorkingDirectory</key>
  <string>%s</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, launchAgentLabel, escapeXML(frpcPath), escapeXML(configPath), escapeXML(rootDir), escapeXML(frp.GetLogPath(rootDir)), escapeXML(frp.GetLogPath(rootDir)))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 LaunchAgent 失败: %w", err)
	}
	return nil
}

func enableWindowsRun(rootDir, frpcPath, configPath string) error {
	command := fmt.Sprintf(`cmd /C cd /D "%s" && "%s" -c "%s"`, rootDir, frpcPath, configPath)
	return exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", windowsRunName, "/t", "REG_SZ", "/d", command, "/f").Run()
}

func enableLinuxAutostart(rootDir, frpcPath, configPath string) error {
	path := linuxDesktopPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建 autostart 目录失败: %w", err)
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=frp-client frpc
Comment=Start frpc for frp-client
Exec=%s -c %s
Path=%s
Terminal=false
X-GNOME-Autostart-enabled=true
`, desktopQuote(frpcPath), desktopQuote(configPath), rootDir)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 Linux 自启动文件失败: %w", err)
	}
	return nil
}

func launchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("Library", "LaunchAgents", launchAgentLabel+".plist")
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func linuxDesktopPath() string {
	if configDir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); configDir != "" {
		return filepath.Join(configDir, "autostart", desktopFileName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "autostart", desktopFileName)
	}
	return filepath.Join(home, ".config", "autostart", desktopFileName)
}

func desktopQuote(value string) string {
	if !strings.ContainsAny(value, " \t\"'\\") {
		return value
	}
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
	return `"` + escaped + `"`
}

func escapeXML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}
