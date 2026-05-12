package frp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetConfigPath(rootDir string) string {
	return filepath.Join(rootDir, "config", "frpc.toml")
}

func ReadConfig(rootDir string) (string, error) {
	configPath := GetConfigPath(rootDir)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			defaultConfig := `serverAddr = "127.0.0.1"
serverPort = 7000

[[proxies]]
name = "example"
type = "tcp"
localIP = "127.0.0.1"
localPort = 8080
remotePort = 8080
`
			dir := filepath.Dir(configPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("创建配置目录失败: %w", err)
			}
			if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
				return "", fmt.Errorf("创建默认配置失败: %w", err)
			}
			return defaultConfig, nil
		}
		return "", fmt.Errorf("读取配置失败: %w", err)
	}
	return string(data), nil
}

func WriteConfig(rootDir, content string) error {
	configPath := GetConfigPath(rootDir)
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}

func ValidateConfig(rootDir, toolsDir string) (string, error) {
	configPath := GetConfigPath(rootDir)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", fmt.Errorf("配置文件 frpc.toml 不存在")
	}

	frpcPath := FindFrpcBinary(toolsDir)
	if frpcPath == "" {
		return "", fmt.Errorf("未找到 frpc 可执行文件，请先安装 frp 客户端")
	}

	cmd := exec.Command(frpcPath, "verify", "-c", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("配置验证失败: %s", msg)
	}

	msg := strings.TrimSpace(string(output))
	if msg == "" {
		msg = "配置验证通过"
	}
	return msg, nil
}
