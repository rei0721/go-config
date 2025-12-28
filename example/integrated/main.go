package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/rei0721/go-config"
)

// AppConfig 应用配置结构
type AppConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		URL      string `yaml:"url"`
		MaxConns int    `yaml:"max_conns"`
	} `yaml:"database"`
	Features struct {
		EnableCache bool   `yaml:"enable_cache"`
		LogLevel    string `yaml:"log_level"`
	} `yaml:"features"`
}

// Validate 实现 Configurable 接口
func (c *AppConfig) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Database.MaxConns <= 0 {
		return fmt.Errorf("database max connections must be positive")
	}
	return nil
}

func main() {
	fmt.Println("=== 集成配置管理器示例 ===")

	// 1. 创建默认配置
	defaultConfig := &AppConfig{
		Server: struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}{
			Host: "localhost",
			Port: 8080,
		},
		Database: struct {
			URL      string `yaml:"url"`
			MaxConns int    `yaml:"max_conns"`
		}{
			URL:      "sqlite://./app.db",
			MaxConns: 10,
		},
		Features: struct {
			EnableCache bool   `yaml:"enable_cache"`
			LogLevel    string `yaml:"log_level"`
		}{
			EnableCache: true,
			LogLevel:    "info",
		},
	}

	// 2. 演示基础管理器使用
	fmt.Println("\n--- 基础管理器示例 ---")
	basicManagerExample(defaultConfig)

	// 3. 演示高级管理器使用
	fmt.Println("\n--- 高级管理器示例 ---")
	advancedManagerExample(defaultConfig)

	// 4. 演示完整集成示例
	fmt.Println("\n--- 完整集成示例 ---")
	fullIntegrationExample(defaultConfig)

	fmt.Println("\n=== 示例完成 ===")
}

// basicManagerExample 基础管理器示例
func basicManagerExample(defaultConfig *AppConfig) {
	// 创建重构后的管理器
	manager := config.NewRefactoredManager(defaultConfig)

	// 构建配置选项
	option, err := config.NewOptionBuilder().
		WithFilename("basic_config.yaml").
		WithFilepath("./configs").
		Build()
	if err != nil {
		log.Printf("创建选项失败: %v", err)
		return
	}

	// 初始化管理器
	ctx := context.Background()
	if err := manager.Initialize(ctx, option); err != nil {
		log.Printf("初始化管理器失败: %v", err)
		return
	}
	defer manager.Close(5 * time.Second)

	// 加载配置
	if err := manager.Load(ctx); err != nil {
		log.Printf("加载配置失败: %v", err)
		return
	}

	// 获取配置
	cfg, err := manager.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return
	}

	fmt.Printf("基础管理器配置: Server=%s:%d, LogLevel=%s\n",
		cfg.Server.Host, cfg.Server.Port, cfg.Features.LogLevel)

	// 获取状态
	status := manager.GetStatus()
	fmt.Printf("管理器状态: 已初始化=%t, 正在监听=%t\n",
		status.Initialized, status.Watching)
}

// advancedManagerExample 高级管理器示例
func advancedManagerExample(defaultConfig *AppConfig) {
	// 创建重构后的管理器
	manager := config.NewRefactoredManager(defaultConfig)

	// 创建配置选项
	option, err := config.NewOptionBuilder().
		WithFilename("advanced_config.yaml").
		WithFilepath("./configs").
		WithDebounceDuration(300 * time.Millisecond).
		WithMaxWorkers(15).
		WithBufferSize(150).
		Build()
	if err != nil {
		log.Printf("创建选项失败: %v", err)
		return
	}

	// 初始化管理器
	ctx := context.Background()
	if err := manager.Initialize(ctx, option); err != nil {
		log.Printf("初始化管理器失败: %v", err)
		return
	}
	defer manager.Close(5 * time.Second)

	// 注册钩子
	manager.RegisterHook(config.HookTypeAfterLoad, &CustomLoadHook{})
	manager.RegisterHook(config.HookTypeAfterUpdate, &CustomUpdateHook{})

	// 加载配置
	if err := manager.Load(ctx); err != nil {
		log.Printf("加载配置失败: %v", err)
		return
	}

	// 启动监听
	if err := manager.StartWatching(ctx); err != nil {
		log.Printf("启动监听失败: %v", err)
	}

	// 获取调度池信息
	if pool := manager.GetSchedulerPool(); pool != nil {
		fmt.Printf("调度池状态: 运行中=%t, 工作器数量=%d, 队列大小=%d\n",
			pool.IsRunning(), pool.GetWorkerCount(), pool.GetCurrentQueueSize())

		// 提交自定义任务
		taskErr := manager.SubmitTask("example-task", config.TaskTypeHook, 0, func(ctx context.Context) error {
			fmt.Println("执行自定义任务")
			return nil
		})
		if taskErr != nil {
			log.Printf("提交任务失败: %v", taskErr)
		}

		// 等待一段时间让任务执行
		time.Sleep(100 * time.Millisecond)
	}

	// 演示配置更新
	updateErr := manager.UpdateConfig(ctx, func(cfg *AppConfig) {
		cfg.Features.LogLevel = "debug"
		cfg.Server.Port = 9090
	})
	if updateErr != nil {
		log.Printf("更新配置失败: %v", updateErr)
	}

	// 获取更新后的配置
	cfg, err := manager.GetConfig()
	if err == nil {
		fmt.Printf("更新后配置: Port=%d, LogLevel=%s\n",
			cfg.Server.Port, cfg.Features.LogLevel)
	}
}

// fullIntegrationExample 完整集成示例
func fullIntegrationExample(defaultConfig *AppConfig) {
	// 创建重构后的管理器
	manager := config.NewRefactoredManager(defaultConfig)

	// 创建配置选项
	option, err := config.NewOptionBuilder().
		WithFilename("integrated_config.yaml").
		WithFilepath("./configs").
		WithDebounceDuration(500 * time.Millisecond).
		WithMaxWorkers(10).
		WithBufferSize(100).
		Build()
	if err != nil {
		log.Printf("创建选项失败: %v", err)
		return
	}

	// 初始化管理器
	ctx := context.Background()
	if err := manager.Initialize(ctx, option); err != nil {
		log.Printf("初始化管理器失败: %v", err)
		return
	}

	// 注册所有类型的钩子
	manager.RegisterHook(config.HookTypeInit, &InitHook{})
	manager.RegisterHook(config.HookTypeBeforeLoad, &BeforeLoadHook{})
	manager.RegisterHook(config.HookTypeAfterLoad, &AfterLoadHook{})
	manager.RegisterHook(config.HookTypeBeforeUpdate, &BeforeUpdateHook{})
	manager.RegisterHook(config.HookTypeAfterUpdate, &AfterUpdateHook{})
	manager.RegisterHook(config.HookTypeError, &ErrorHook{})

	// 注册配置变更处理器
	changeHandler := func(ctx context.Context, oldConfig, newConfig *AppConfig) error {
		fmt.Printf("配置变更: LogLevel %s -> %s\n",
			oldConfig.Features.LogLevel, newConfig.Features.LogLevel)
		return nil
	}

	// 加载配置
	if err := manager.Load(ctx, changeHandler); err != nil {
		log.Printf("加载配置失败: %v", err)
		return
	}

	// 启动监听
	if err := manager.StartWatching(ctx); err != nil {
		log.Printf("启动监听失败: %v", err)
	}

	// 获取拦截统计信息
	stats := manager.GetInterceptionStats()
	fmt.Printf("拦截统计: %+v\n", stats)

	// 演示配置存储功能
	if store := manager.GetConfigStore(); store != nil {
		// 创建快照
		version, err := store.CreateSnapshot("演示快照")
		if err == nil {
			fmt.Printf("创建快照成功，版本: %d\n", version)
		}

		// 获取统计信息
		storeStats := store.GetStats()
		fmt.Printf("配置存储统计: 版本=%d, 快照数量=%d\n",
			storeStats.CurrentVersion, storeStats.SnapshotCount)
	}

	// 演示优雅关闭
	fmt.Println("按 Ctrl+C 进行优雅关闭演示...")
	gracefulShutdown(manager)
}

// gracefulShutdown 优雅关闭示例
func gracefulShutdown(manager *config.RefactoredManager[AppConfig]) {
	// 创建信号通道
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动一个goroutine来模拟信号（用于演示）
	go func() {
		time.Sleep(2 * time.Second)
		sigChan <- syscall.SIGINT
	}()

	// 等待信号
	<-sigChan
	fmt.Println("\n收到关闭信号，开始优雅关闭...")

	// 停止监听
	if err := manager.StopWatching(3 * time.Second); err != nil {
		log.Printf("停止监听失败: %v", err)
	}

	// 关闭管理器
	if err := manager.Close(5 * time.Second); err != nil {
		log.Printf("关闭管理器失败: %v", err)
	}

	fmt.Println("优雅关闭完成")
}

// 钩子实现

type CustomLoadHook struct{}

func (h *CustomLoadHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("配置加载完成: %s\n", ctx.Message)
	return nil
}

func (h *CustomLoadHook) Priority() int { return 0 }
func (h *CustomLoadHook) IsAsync() bool { return false }

type CustomUpdateHook struct{}

func (h *CustomUpdateHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("配置更新完成: %s\n", ctx.Message)
	return nil
}

func (h *CustomUpdateHook) Priority() int { return 0 }
func (h *CustomUpdateHook) IsAsync() bool { return true }

type InitHook struct{}

func (h *InitHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("系统初始化: %s\n", ctx.Message)
	return nil
}

func (h *InitHook) Priority() int { return 0 }
func (h *InitHook) IsAsync() bool { return false }

type BeforeLoadHook struct{}

func (h *BeforeLoadHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("配置加载前: %s\n", ctx.Message)
	return nil
}

func (h *BeforeLoadHook) Priority() int { return 0 }
func (h *BeforeLoadHook) IsAsync() bool { return false }

type AfterLoadHook struct{}

func (h *AfterLoadHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("配置加载后: %s\n", ctx.Message)
	return nil
}

func (h *AfterLoadHook) Priority() int { return 0 }
func (h *AfterLoadHook) IsAsync() bool { return false }

type BeforeUpdateHook struct{}

func (h *BeforeUpdateHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("配置更新前: %s\n", ctx.Message)
	return nil
}

func (h *BeforeUpdateHook) Priority() int { return 0 }
func (h *BeforeUpdateHook) IsAsync() bool { return false }

type AfterUpdateHook struct{}

func (h *AfterUpdateHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("配置更新后: %s\n", ctx.Message)
	return nil
}

func (h *AfterUpdateHook) Priority() int { return 0 }
func (h *AfterUpdateHook) IsAsync() bool { return false }

type ErrorHook struct{}

func (h *ErrorHook) Handle(ctx *config.HookContext) error {
	if ctx.Error != nil {
		fmt.Printf("系统错误: %s - %v\n", ctx.Message, ctx.Error)
	}
	return nil
}

func (h *ErrorHook) Priority() int { return 0 }
func (h *ErrorHook) IsAsync() bool { return true }
