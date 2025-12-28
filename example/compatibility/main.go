package main

import (
	"context"
	"fmt"
	"log"
	"time"

	config "github.com/rei0721/go-config"
)

// AppConfig 应用配置结构
type AppConfig struct {
	App struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		Debug   bool   `yaml:"debug"`
	} `yaml:"app"`
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Driver   string `yaml:"driver"`
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Database string `yaml:"database"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"database"`
}

func main() {
	fmt.Println("=== 向后兼容性演示 ===")

	// 创建默认配置
	defaultConfig := &AppConfig{}
	defaultConfig.App.Name = "MyApp"
	defaultConfig.App.Version = "1.0.0"
	defaultConfig.App.Debug = false
	defaultConfig.Server.Host = "localhost"
	defaultConfig.Server.Port = 8080
	defaultConfig.Database.Driver = "mysql"
	defaultConfig.Database.Host = "localhost"
	defaultConfig.Database.Port = 3306
	defaultConfig.Database.Database = "myapp"
	defaultConfig.Database.Username = "user"
	defaultConfig.Database.Password = "password"

	// 演示1: 传统API使用方式（完全兼容）
	fmt.Println("\n--- 演示1: 传统API使用方式 ---")
	demonstrateTraditionalAPI(defaultConfig)

	// 演示2: 兼容性层使用方式（推荐迁移方式）
	fmt.Println("\n--- 演示2: 兼容性层使用方式 ---")
	demonstrateCompatibilityLayer(defaultConfig)

	// 演示3: 渐进式迁移（启用高级功能）
	fmt.Println("\n--- 演示3: 渐进式迁移 ---")
	demonstrateProgressiveMigration(defaultConfig)

	// 演示4: 完全新API使用方式
	fmt.Println("\n--- 演示4: 完全新API使用方式 ---")
	demonstrateNewAPI(defaultConfig)

	// 演示5: 兼容性检查
	fmt.Println("\n--- 演示5: 兼容性检查 ---")
	demonstrateCompatibilityCheck()

	fmt.Println("\n=== 演示完成 ===")
}

// demonstrateTraditionalAPI 演示传统API使用方式
func demonstrateTraditionalAPI(defaultConfig *AppConfig) {
	fmt.Println("使用传统Manager API...")

	// 创建传统管理器
	manager := config.NewManager(defaultConfig)

	// 传统初始化方式
	ctx := context.Background()
	err := manager.Init(ctx)
	if err != nil {
		log.Printf("初始化失败: %v", err)
		return
	}

	// 传统获取配置方式
	cfg, err := manager.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return
	}

	fmt.Printf("应用名称: %s, 版本: %s\n", cfg.App.Name, cfg.App.Version)
	fmt.Printf("服务器: %s:%d\n", cfg.Server.Host, cfg.Server.Port)

	// 传统更新配置方式
	err = manager.UpdateField(ctx, func(c *AppConfig) {
		c.App.Debug = true
		c.Server.Port = 9090
	})
	if err != nil {
		log.Printf("更新配置失败: %v", err)
	} else {
		fmt.Println("配置更新成功（传统方式）")
	}

	// 传统钩子设置方式
	manager.SetHook(config.Info, func(ctx config.HookContext) {
		fmt.Printf("传统钩子: %s\n", ctx.Message)
	})

	fmt.Println("传统API演示完成")
}

// demonstrateCompatibilityLayer 演示兼容性层使用方式
func demonstrateCompatibilityLayer(defaultConfig *AppConfig) {
	fmt.Println("使用兼容性层...")

	// 使用兼容性层创建管理器
	manager := config.NewManager(defaultConfig)

	// 兼容的初始化方式
	ctx := context.Background()
	err := manager.Init(ctx)
	if err != nil {
		log.Printf("初始化失败: %v", err)
		return
	}

	// 兼容的获取配置方式
	cfg, err := manager.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return
	}

	fmt.Printf("应用名称: %s, 版本: %s\n", cfg.App.Name, cfg.App.Version)

	// 兼容的更新配置方式
	err = manager.UpdateField(ctx, func(c *AppConfig) {
		c.App.Version = "1.1.0"
	})
	if err != nil {
		log.Printf("更新配置失败: %v", err)
	} else {
		fmt.Println("配置更新成功（兼容性层）")
	}

	// 获取状态
	status := manager.GetStatus()
	fmt.Printf("管理器状态: 已初始化=%t\n", status.Initialized)

	fmt.Println("兼容性层演示完成")
}

// demonstrateProgressiveMigration 演示渐进式迁移
func demonstrateProgressiveMigration(defaultConfig *AppConfig) {
	fmt.Println("演示渐进式迁移...")

	// 从传统管理器开始
	manager := config.NewManager(defaultConfig)
	ctx := context.Background()
	err := manager.Init(ctx)
	if err != nil {
		log.Printf("初始化失败: %v", err)
		return
	}

	fmt.Printf("初始模式: 重构模式=%t\n", manager.IsRefactoredMode())

	// 启用重构模式
	manager.EnableRefactoredMode()
	fmt.Printf("启用后: 重构模式=%t\n", manager.IsRefactoredMode())

	// 现在可以使用高级功能
	err = manager.StartWatchingWithOptions(ctx)
	if err != nil {
		log.Printf("启动监听失败: %v", err)
	} else {
		fmt.Println("文件监听已启动")
	}

	// 获取重构管理器
	refactoredManager := manager.GetRefactoredManager()
	if refactoredManager != nil {
		// 使用调度池
		pool := refactoredManager.GetSchedulerPool()
		if pool != nil {
			fmt.Printf("调度池状态: 运行中=%t, 工作器数量=%d\n",
				pool.IsRunning(), pool.GetWorkerCount())
		}

		// 获取拦截统计
		stats := refactoredManager.GetInterceptionStats()
		fmt.Printf("拦截统计: %+v\n", stats)
	}

	// 停止监听
	defer manager.StopWatching(5 * time.Second)

	fmt.Println("渐进式迁移演示完成")
}

// demonstrateNewAPI 演示完全新API使用方式
func demonstrateNewAPI(defaultConfig *AppConfig) {
	fmt.Println("使用完全新API...")

	// 使用新的工厂函数创建管理器
	manager, err := config.CreateIntegratedManager(
		defaultConfig,
		"app_config.yaml",
		"./configs",
	)
	if err != nil {
		log.Printf("创建管理器失败: %v", err)
		return
	}

	// 使用新的线程安全方法
	cfg, err := manager.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return
	}

	fmt.Printf("应用名称: %s, 版本: %s\n", cfg.App.Name, cfg.App.Version)

	// 使用新的线程安全更新方法
	ctx := context.Background()
	err = manager.UpdateField(ctx, func(c *AppConfig) {
		c.App.Debug = true
		c.Server.Port = 8888
	})
	if err != nil {
		log.Printf("更新配置失败: %v", err)
	} else {
		fmt.Println("配置更新成功（新API）")
	}

	// 使用新的钩子系统
	if manager.IsRefactoredMode() {
		refactoredManager := manager.GetRefactoredManager()
		if refactoredManager != nil {
			hook := &CustomHook{name: "demo-hook"}
			err = refactoredManager.RegisterHook(config.HookTypeInfo, hook)
			if err != nil {
				log.Printf("注册钩子失败: %v", err)
			} else {
				fmt.Println("新钩子注册成功")
			}
		}
	}

	// 提交自定义任务（如果支持）
	if refactoredManager := manager.GetRefactoredManager(); refactoredManager != nil {
		err = refactoredManager.SubmitTask("demo-task", config.TaskTypeHook, 0, func(ctx context.Context) error {
			fmt.Println("执行自定义任务（新API）")
			return nil
		})
		if err != nil {
			log.Printf("提交任务失败: %v", err)
		}
	}

	// 等待任务执行
	time.Sleep(100 * time.Millisecond)

	// 关闭管理器
	defer manager.Close(5 * time.Second)

	fmt.Println("新API演示完成")
}

// demonstrateCompatibilityCheck 演示兼容性检查
func demonstrateCompatibilityCheck() {
	fmt.Println("执行兼容性检查...")

	// 检查API版本信息（模拟实现）
	fmt.Printf("API版本: %s\n", "2.0.0")
	fmt.Printf("新功能数量: %d\n", 5)
	fmt.Printf("废弃功能数量: %d\n", 2)
	fmt.Printf("破坏性变更数量: %d\n", 1)

	// 检查兼容性（模拟实现）
	fmt.Printf("兼容性检查: 向后兼容=true, 新功能可用=true\n")

	// 验证特定版本兼容性（模拟实现）
	compatible := true
	fmt.Printf("版本1.5.0兼容性: %t\n", compatible)

	// 获取迁移指南（模拟实现）
	fmt.Printf("迁移指南: 从%s到%s, %d个步骤\n", "1.0.0", "2.0.0", 3)

	// 获取废弃警告（模拟实现）
	fmt.Printf("废弃警告数量: %d\n", 2)
	fmt.Printf("  - %s: %s\n", "OldFunction", "请使用NewFunction替代")
	fmt.Printf("  - %s: %s\n", "LegacyAPI", "将在下个版本中移除")

	fmt.Println("兼容性检查完成")
}

// CustomHook 自定义钩子实现
type CustomHook struct {
	name string
}

func (h *CustomHook) Handle(ctx *config.HookContext) error {
	fmt.Printf("自定义钩子[%s]: %s\n", h.name, ctx.Message)
	return nil
}

func (h *CustomHook) Priority() int {
	return 0
}

func (h *CustomHook) IsAsync() bool {
	return false
}
