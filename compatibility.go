package config

import (
	"context"
	"fmt"
	"time"
)

// CompatibilityLayer 兼容性层
// 提供向后兼容的API，确保现有代码可以无缝迁移到新架构
type CompatibilityLayer[T Configurable] struct {
	manager *Manager[T]
}

// NewCompatibilityLayer 创建兼容性层
func NewCompatibilityLayer[T Configurable](manager *Manager[T]) *CompatibilityLayer[T] {
	return &CompatibilityLayer[T]{
		manager: manager,
	}
}

// 传统API兼容性方法

// Load 加载配置（兼容旧版本API）
func (cl *CompatibilityLayer[T]) Load(handles ...HandlerFunc) error {
	ctx := context.Background()
	return cl.manager.Init(ctx, handles...)
}

// GetConfig 获取配置（兼容旧版本API）
func (cl *CompatibilityLayer[T]) GetConfig() (*T, error) {
	return cl.manager.GetConfig()
}

// UpdateField 更新字段（兼容旧版本API）
func (cl *CompatibilityLayer[T]) UpdateField(updateFunc func(*T)) error {
	ctx := context.Background()
	return cl.manager.UpdateField(ctx, updateFunc)
}

// SetHook 设置钩子（兼容旧版本API）
func (cl *CompatibilityLayer[T]) SetHook(pattern HookPattern, handler HookHandlerFunc) *CompatibilityLayer[T] {
	cl.manager.SetHook(pattern, handler)
	return cl
}

// 新增的便捷方法

// EnableAdvancedFeatures 启用高级功能
// 这个方法帮助用户从传统模式迁移到重构模式
func (cl *CompatibilityLayer[T]) EnableAdvancedFeatures() *CompatibilityLayer[T] {
	cl.manager.EnableRefactoredMode()
	return cl
}

// StartWatching 启动文件监听（简化版）
func (cl *CompatibilityLayer[T]) StartWatching() error {
	ctx := context.Background()
	return cl.manager.StartWatchingWithOptions(ctx)
}

// StopWatching 停止文件监听（简化版）
func (cl *CompatibilityLayer[T]) StopWatching() error {
	return cl.manager.StopWatching(5 * time.Second)
}

// GetStatus 获取管理器状态
func (cl *CompatibilityLayer[T]) GetStatus() *ManagerStatus {
	return cl.manager.GetStatus()
}

// Close 关闭管理器
func (cl *CompatibilityLayer[T]) Close() error {
	return cl.manager.Close(5 * time.Second)
}

// 迁移辅助方法

// MigrateToRefactored 迁移到重构模式
func (cl *CompatibilityLayer[T]) MigrateToRefactored(configPath string) error {
	option, err := NewOptionBuilder().
		WithFilename("config.yaml").
		WithFilepath(configPath).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build migration option: %w", err)
	}

	ctx := context.Background()
	return cl.manager.MigrateToRefactored(ctx, option)
}

// GetAdvancedManager 获取高级管理器功能
// 返回重构后的管理器，提供高级功能访问
func (cl *CompatibilityLayer[T]) GetAdvancedManager() *RefactoredManager[T] {
	return cl.manager.GetRefactoredManager()
}

// IsUsingAdvancedFeatures 检查是否使用高级功能
func (cl *CompatibilityLayer[T]) IsUsingAdvancedFeatures() bool {
	return cl.manager.IsRefactoredMode()
}

// 全局兼容性函数

// Default 创建默认管理器（兼容旧版本）
func Default[T Configurable](defaultConfig *T) *CompatibilityLayer[T] {
	manager := NewManager(defaultConfig)
	return NewCompatibilityLayer(manager)
}

// NewWithOption 使用选项创建管理器（兼容旧版本）
func NewWithOption[T Configurable](defaultConfig *T, option *Option) *CompatibilityLayer[T] {
	manager := NewManager(defaultConfig)

	// 转换旧选项到新选项
	newOption, err := NewOptionBuilder().
		WithFilename(option.Filename.ToValue()).
		WithFilepath(option.Filepath.ToValue()).
		WithDebounceDuration(option.DebounceDur.ToValue()).
		Build()

	if err == nil {
		ctx := context.Background()
		manager.InitWithOptions(ctx, newOption)
	}

	return NewCompatibilityLayer(manager)
}

// APIVersionInfo API版本信息
type APIVersionInfo struct {
	Version       string            `json:"version"`
	Features      []string          `json:"features"`
	Deprecated    []string          `json:"deprecated"`
	Breaking      []string          `json:"breaking"`
	Migration     map[string]string `json:"migration"`
	Compatibility bool              `json:"compatibility"`
}

// GetAPIVersionInfo 获取API版本信息
func GetAPIVersionInfo() *APIVersionInfo {
	return &APIVersionInfo{
		Version: "2.0.0",
		Features: []string{
			"Refactored Architecture",
			"Scheduler Pool",
			"Thread-Safe Config Store",
			"Advanced File Watcher",
			"Hook System",
			"Viper Goroutine Interception",
			"Property-Based Testing Support",
		},
		Deprecated: []string{
			"Direct Manager.UpdateField usage (use UpdateConfigSafe instead)",
			"Direct Manager.GetConfig usage (use GetConfigSafe instead)",
			"Old Hook API (use new HookHandler interface)",
		},
		Breaking: []string{
			"Manager struct fields are now private",
			"Hook system API changed",
			"Option system completely refactored",
		},
		Migration: map[string]string{
			"Manager.UpdateField": "Manager.UpdateConfigSafe",
			"Manager.GetConfig":   "Manager.GetConfigSafe",
			"Old Hook API":        "New HookHandler interface",
			"Old Option API":      "New OptionBuilder pattern",
		},
		Compatibility: true,
	}
}

// MigrationGuide 迁移指南
type MigrationGuide struct {
	FromVersion string                      `json:"from_version"`
	ToVersion   string                      `json:"to_version"`
	Steps       []MigrationStep             `json:"steps"`
	Examples    map[string]MigrationExample `json:"examples"`
}

// MigrationStep 迁移步骤
type MigrationStep struct {
	Step        int    `json:"step"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Code        string `json:"code,omitempty"`
	Required    bool   `json:"required"`
}

// MigrationExample 迁移示例
type MigrationExample struct {
	Title       string `json:"title"`
	OldCode     string `json:"old_code"`
	NewCode     string `json:"new_code"`
	Description string `json:"description"`
}

// GetMigrationGuide 获取迁移指南
func GetMigrationGuide() *MigrationGuide {
	return &MigrationGuide{
		FromVersion: "1.x",
		ToVersion:   "2.0.0",
		Steps: []MigrationStep{
			{
				Step:        1,
				Title:       "更新导入",
				Description: "确保使用正确的包导入路径",
				Code:        `import "github.com/rei0721/go-config"`,
				Required:    true,
			},
			{
				Step:        2,
				Title:       "使用兼容性层",
				Description: "使用兼容性层进行平滑迁移",
				Code:        `manager := config.Default(defaultConfig)`,
				Required:    false,
			},
			{
				Step:        3,
				Title:       "启用高级功能",
				Description: "逐步启用新的高级功能",
				Code:        `manager.EnableAdvancedFeatures()`,
				Required:    false,
			},
			{
				Step:        4,
				Title:       "迁移到新API",
				Description: "将代码迁移到新的API",
				Code:        `manager.MigrateToRefactored("./configs")`,
				Required:    false,
			},
			{
				Step:        5,
				Title:       "测试和验证",
				Description: "测试迁移后的功能",
				Required:    true,
			},
		},
		Examples: map[string]MigrationExample{
			"basic_usage": {
				Title: "基本使用迁移",
				OldCode: `manager := config.NewManager(defaultConfig)
err := manager.Load()
cfg, err := manager.GetConfig()`,
				NewCode: `manager := config.Default(defaultConfig)
err := manager.Load()
cfg, err := manager.GetConfig()`,
				Description: "基本API保持不变，使用兼容性层",
			},
			"advanced_features": {
				Title:   "高级功能迁移",
				OldCode: `// 旧版本没有这些功能`,
				NewCode: `manager := config.Default(defaultConfig)
manager.EnableAdvancedFeatures()
err := manager.StartWatching()
pool := manager.GetAdvancedManager().GetSchedulerPool()`,
				Description: "启用新的高级功能",
			},
			"hook_system": {
				Title: "钩子系统迁移",
				OldCode: `manager.SetHook(config.Info, func(ctx config.HookContext) {
    log.Println(ctx.Message)
})`,
				NewCode: `type MyHook struct{}
func (h *MyHook) Handle(ctx *config.HookContext) error {
    log.Println(ctx.Message)
    return nil
}
func (h *MyHook) Priority() int { return 0 }
func (h *MyHook) IsAsync() bool { return false }

manager.GetAdvancedManager().RegisterHook(config.HookTypeInfo, &MyHook{})`,
				Description: "新的钩子系统提供更好的类型安全和功能",
			},
		},
	}
}

// DeprecationWarning 废弃警告
type DeprecationWarning struct {
	Function    string `json:"function"`
	Version     string `json:"version"`
	Replacement string `json:"replacement"`
	Message     string `json:"message"`
}

// GetDeprecationWarnings 获取废弃警告
func GetDeprecationWarnings() []DeprecationWarning {
	return []DeprecationWarning{
		{
			Function:    "Manager.UpdateField",
			Version:     "2.0.0",
			Replacement: "Manager.UpdateConfigSafe",
			Message:     "UpdateField is deprecated, use UpdateConfigSafe for better thread safety",
		},
		{
			Function:    "Manager.GetConfig",
			Version:     "2.0.0",
			Replacement: "Manager.GetConfigSafe",
			Message:     "GetConfig is deprecated, use GetConfigSafe for better thread safety",
		},
		{
			Function:    "Old Hook API",
			Version:     "2.0.0",
			Replacement: "HookHandler interface",
			Message:     "Old hook API is deprecated, use new HookHandler interface for better functionality",
		},
	}
}

// CheckCompatibility 检查兼容性
func CheckCompatibility() map[string]interface{} {
	return map[string]interface{}{
		"api_version":         "2.0.0",
		"backward_compatible": true,
		"migration_required":  false,
		"deprecated_features": len(GetDeprecationWarnings()),
		"new_features":        len(GetAPIVersionInfo().Features),
		"breaking_changes":    len(GetAPIVersionInfo().Breaking),
	}
}

// ValidateCompatibility 验证兼容性
func ValidateCompatibility(version string) (bool, []string) {
	var issues []string
	compatible := true

	// 检查版本兼容性
	if version < "1.0.0" {
		compatible = false
		issues = append(issues, "Version too old, minimum supported version is 1.0.0")
	}

	// 检查功能兼容性
	warnings := GetDeprecationWarnings()
	for _, warning := range warnings {
		issues = append(issues, fmt.Sprintf("Deprecated: %s - %s", warning.Function, warning.Message))
	}

	return compatible, issues
}

// AutoMigrate 自动迁移
func AutoMigrate[T Configurable](oldManager interface{}) (*Manager[T], error) {
	// 这里可以实现自动迁移逻辑
	// 由于Go的类型系统限制，这里提供一个基本框架
	return nil, fmt.Errorf("auto migration not implemented, please use manual migration")
}

// CreateCompatibleManager 创建兼容的管理器
// 这是推荐的创建方式，提供最佳的兼容性
func CreateCompatibleManager[T Configurable](defaultConfig *T, enableAdvanced bool) *CompatibilityLayer[T] {
	manager := NewManager(defaultConfig)

	if enableAdvanced {
		manager.EnableRefactoredMode()
	}

	return NewCompatibilityLayer(manager)
}
