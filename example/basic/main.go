package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rei0721/go-config"
)

// Logger 简单日志实现
type Logger struct{}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) log(level, format string, args ...interface{}) {
	fmt.Printf("[LOG] [%s] %s\n", level, fmt.Sprintf(format, args...))
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log("DEBUG", format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log("INFO", format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log("WARN", format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log("ERROR", format, args...)
}

// App 应用基础配置
type App struct {
	Name        string `yaml:"name" mapstructure:"name"`
	Version     string `yaml:"version" mapstructure:"version"`
	Description string `yaml:"description" mapstructure:"description"`
}

// Configs 配置结构
type Configs struct {
	App App `yaml:"app" mapstructure:"app"`
}

// Validate 实现 Configurable 接口
func (c *Configs) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	return nil
}

// InfoHook 信息钩子实现
type InfoHook struct {
	logger *Logger
}

func (h *InfoHook) Handle(ctx *config.HookContext) error {
	h.logger.Info("钩子触发: %s", ctx.Message)
	return nil
}

func (h *InfoHook) Priority() int { return 0 }
func (h *InfoHook) IsAsync() bool { return false }

// DebugHook 调试钩子实现
type DebugHook struct {
	logger *Logger
}

func (h *DebugHook) Handle(ctx *config.HookContext) error {
	h.logger.Debug("调试信息: %s", ctx.Message)
	return nil
}

func (h *DebugHook) Priority() int { return 0 }
func (h *DebugHook) IsAsync() bool { return false }

// ErrorHook 错误钩子实现
type ErrorHook struct {
	logger *Logger
}

func (h *ErrorHook) Handle(ctx *config.HookContext) error {
	if ctx.Error != nil {
		h.logger.Error("错误: %s - %v", ctx.Message, ctx.Error)
	} else {
		h.logger.Error("错误: %s", ctx.Message)
	}
	return nil
}

func (h *ErrorHook) Priority() int { return 0 }
func (h *ErrorHook) IsAsync() bool { return true }

func main() {
	// 实例化日志
	logger := NewLogger()

	// 创建默认配置
	defaultConfig := &Configs{
		App: App{
			Name:        "BasicApp",
			Version:     "1.0.0",
			Description: "基础配置管理示例应用",
		},
	}

	// 创建重构后的配置管理器
	manager := config.NewRefactoredManager(defaultConfig)

	// 构建配置选项
	option, err := config.NewOptionBuilder().
		WithFilename("config.dev.yaml").
		WithFilepath("./configs").
		WithDebounceDuration(800 * time.Millisecond).
		WithMaxWorkers(5).
		WithBufferSize(50).
		Build()
	if err != nil {
		logger.Error("构建配置选项失败: %v", err)
		return
	}

	// 初始化管理器
	ctx := context.Background()
	if err := manager.Initialize(ctx, option); err != nil {
		logger.Error("初始化配置管理器失败: %v", err)
		return
	}
	defer manager.Close(5 * time.Second)

	// 注册钩子
	manager.RegisterHook(config.HookTypeInit, &InfoHook{logger: logger})
	manager.RegisterHook(config.HookTypeDebug, &DebugHook{logger: logger})
	manager.RegisterHook(config.HookTypeInfo, &InfoHook{logger: logger})
	manager.RegisterHook(config.HookTypeWarn, &InfoHook{logger: logger})
	manager.RegisterHook(config.HookTypeError, &ErrorHook{logger: logger})

	// 注册配置变更处理器
	changeHandler := func(ctx context.Context, oldConfig, newConfig *Configs) error {
		logger.Info("配置文件更新了, app: %s -> %s", oldConfig.App.Name, newConfig.App.Name)
		return nil
	}

	// 加载配置
	if err := manager.Load(ctx, changeHandler); err != nil {
		logger.Error("加载配置失败: %v", err)
		return
	}

	logger.Info("初始化成功")

	// 获取当前配置
	cfg, err := manager.GetConfig()
	if err != nil {
		logger.Error("获取配置失败: %v", err)
		return
	}

	logger.Info("当前配置: Name=%s, Version=%s", cfg.App.Name, cfg.App.Version)

	// 启动文件监听
	if err := manager.StartWatching(ctx); err != nil {
		logger.Warn("启动文件监听失败: %v", err)
	} else {
		logger.Info("文件监听已启动")
	}

	// 演示动态配置更新
	go func() {
		time.Sleep(2 * time.Second)
		logger.Info("开始更新配置...")

		err := manager.UpdateConfig(ctx, func(config *Configs) {
			config.App.Name = "UpdatedApp"
			config.App.Version = "2.0.0"
		})
		if err != nil {
			logger.Error("更新配置失败: %v", err)
		} else {
			logger.Info("配置更新成功")
		}
	}()

	// 获取管理器状态
	status := manager.GetStatus()
	logger.Info("管理器状态: 已初始化=%t, 正在监听=%t, 版本=%d",
		status.Initialized, status.Watching, status.ConfigVersion)

	// 保持程序运行一段时间以观察效果
	time.Sleep(5 * time.Second)

	logger.Info("程序即将退出...")
}
