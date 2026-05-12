package system

import (
	"fmt"
	"runtime"
)

type SystemInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

func GetSystemInfo() SystemInfo {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "darwin":
		goos = "darwin"
	case "windows":
		goos = "windows"
	case "linux":
		goos = "linux"
	}

	switch goarch {
	case "amd64":
		goarch = "amd64"
	case "arm64":
		goarch = "arm64"
	}

	return SystemInfo{
		OS:   goos,
		Arch: goarch,
	}
}

func GetPlatformDir() string {
	info := GetSystemInfo()
	return fmt.Sprintf("%s", info.OS)
}

func GetFrpcExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}
