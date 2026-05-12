package frp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func DefaultConfig() string {
	return `serverAddr = "127.0.0.1"
serverPort = 7000

[[proxies]]
name = "example"
type = "tcp"
localIP = "127.0.0.1"
localPort = 8080
remotePort = 8080
`
}

func ReadConfig(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("读取配置失败: %w", err)
	}
	return string(data), nil
}

func WriteConfig(configPath, content string) error {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}

func ValidateConfig(configPath, toolPath string) (string, error) {
	if _, err := os.Stat(configPath); err != nil {
		return "", fmt.Errorf("配置文件不存在: %s", configPath)
	}
	if _, err := os.Stat(toolPath); err != nil {
		return "", fmt.Errorf("未找到 frpc 可执行文件: %s", toolPath)
	}

	cmd := exec.Command(toolPath, "verify", "-c", configPath)
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
