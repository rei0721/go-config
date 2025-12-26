package config

import (
	"path/filepath"
)

// filepathABs 返回配置文件路径
func filepathAbs(p string) string {
	path, _ := filepath.Abs(p)
	return path
}
