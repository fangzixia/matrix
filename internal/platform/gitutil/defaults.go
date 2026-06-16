package gitutil

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultSSHKeyPath 返回当前运行环境的默认 SSH 私钥路径。
func DefaultSSHKeyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "id_rsa")
}

// PlatformLabel 返回面向用户展示的操作系统名称。
func PlatformLabel(goos string) string {
	switch goos {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	default:
		if goos == "" {
			return "Linux"
		}
		return "Linux"
	}
}

// ServerPlatform 返回当前服务进程所在操作系统（runtime.GOOS）。
func ServerPlatform() string {
	return runtime.GOOS
}
