package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func (m *Manager[T]) initializeValidateSetting(ok bool) {
	m.initializeValidate = ok
}

func (m *Manager[T]) InitializeValidate() bool {
	return m.initializeValidate
}

// GetConfig 获取配置
// 返回值：
//
//	*Config: 配置副本
//	error: 获取过程中的错误
func (m *Manager[T]) GetConfig() (*T, error) {
	//m := Default()
	m.rwMutex.RLock()
	defer m.rwMutex.RUnlock()

	if m.config == nil {
		return nil, fmt.Errorf("config not initialized")
	}

	configCopy := *m.config
	return &configCopy, nil
}

// ConfigPath 返回应用配置文件路径
//
// appName: 应用名，比如 ".qwq"
//
// fileName: 配置文件名，比如 "config.toml"
//
// noUserPath: 固定配置路径 如 windows C:\Users\<用户>\<应用名>\<配置文件名>、linux \root\<应用名>\<配置文件名>
func ConfigPath(appName, fileName string, noUserPath bool) string {
	var baseDir string // 定义一个变量 baseDir，用来存储基础目录路径

	if noUserPath {
		p := filepath.Join(appName, fileName)
		path, _ := filepath.Abs(p)

		return path
	}

	// 根据操作系统做不同处理
	switch runtime.GOOS {
	case "windows":
		// -----------------------------
		// Windows 系统
		// -----------------------------
		// Windows 系统有一个环境变量 APPDATA，通常指：
		// C:\Users\<用户名>\AppData\Roaming
		// baseDir = os.Getenv("APPDATA")

		// 如果 APPDATA 环境变量为空（极少数情况）， fallback 使用用户目录
		// if baseDir == "" {
		home, _ := os.UserHomeDir()            // 获取当前用户主目录，如 C:\Users\xiaolin
		baseDir = filepath.Join(home, appName) // 拼接成 C:\Users\xiaolin\<appName>
		// }

	case "linux", "darwin":
		// -----------------------------
		// Linux 或 macOS 系统
		// -----------------------------
		if os.Geteuid() == 0 {
			// 如果当前用户是 root 用户（Linux/macOS 可用 os.Geteuid() 判断）
			// 系统级配置通常放 /etc/<appName>
			baseDir = filepath.Join("/etc", appName)
		} else {
			// 普通用户配置，放在 $HOME/.config/<appName>
			home, _ := os.UserHomeDir() // 获取当前用户主目录
			baseDir = filepath.Join(home, ".config", appName)
		}

	default:
		// -----------------------------
		// 其他平台（不常用）
		// -----------------------------
		home, _ := os.UserHomeDir()                       // 获取用户目录
		baseDir = filepath.Join(home, ".config", appName) // 默认放 ~/.config/<appName>
	}

	return filepath.Join(baseDir, fileName)
	// 如果想返回完整配置文件路径，应使用这一行，把文件名拼接上
}
