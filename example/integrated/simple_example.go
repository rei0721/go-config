package main

import (
	"context"
	"fmt"
	"log"
	"time"

	config "github.com/rei0721/go-config"
)

// ExampleConfig 示例配置结构
type ExampleConfig struct {
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

func simpleMain() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("程序发生panic: %v\n", r)
		}
	}()

	fmt.Println("=== 集成配置管理器示例 ===")
	fmt.Println("程序开始执行...")

	// 1. 创建默认配置
	defaultConfig := &ExampleConfig{}
	defaultConfig.Server.Host = "localhost"
	defaultConfig.Server.Port = 8080
	defaultConfig.Database.URL = "sqlite://./app.db"
	defaultConfig.Database.MaxConns = 10
	defaultConfig.Features.EnableCache = true
	defaultConfig.Features.LogLevel = "info"

	fmt.Println("默认配置创建完成")

	// 2. 使用集成工厂创建管理器
	fmt.Println("\n--- 创建集成管理器 ---")
	manager, err := config.CreateIntegratedManager(
		defaultConfig,
		"simple_config.yaml",
		"./configs",
	)
	if err != nil {
		log.Printf("创建管理器失败: %v", err)
		fmt.Printf("错误详情: %v\n", err)
		return
	}

	fmt.Println("管理器创建成功")

	// 3. 获取配置
	fmt.Println("\n--- 获取配置 ---")
	cfg, err := manager.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		fmt.Printf("错误详情: %v\n", err)
		return
	}

	fmt.Printf("配置信息: Server=%s:%d, LogLevel=%s\n",
		cfg.Server.Host, cfg.Server.Port, cfg.Features.LogLevel)

	// 4. 获取管理器状态
	fmt.Println("\n--- 管理器状态 ---")
	status := manager.GetStatus()
	fmt.Printf("管理器状态: 已初始化=%t, 重构模式=%t\n",
		status.Initialized, manager.IsRefactoredMode())

	// 5. 演示调度池功能（如果可用）
	fmt.Println("\n--- 调度池功能 ---")
	if manager.IsRefactoredMode() {
		refactoredManager := manager.GetRefactoredManager()
		if refactoredManager != nil {
			pool := refactoredManager.GetSchedulerPool()
			if pool != nil {
				fmt.Printf("调度池状态: 运行中=%t, 工作器数量=%d\n",
					pool.IsRunning(), pool.GetWorkerCount())

				// 提交一个简单任务
				err := refactoredManager.SubmitTask("demo-task", config.TaskTypeHook, 0, func(ctx context.Context) error {
					fmt.Println("  -> 执行演示任务")
					return nil
				})
				if err != nil {
					log.Printf("提交任务失败: %v", err)
				} else {
					fmt.Println("任务提交成功")
					time.Sleep(100 * time.Millisecond) // 等待任务执行
				}
			} else {
				fmt.Println("调度池不可用")
			}
		} else {
			fmt.Println("重构管理器不可用")
		}
	} else {
		fmt.Println("未启用重构模式")
	}

	// 6. 演示配置更新
	fmt.Println("\n--- 配置更新演示 ---")
	ctx := context.Background()
	updateErr := manager.UpdateField(ctx, func(cfg *ExampleConfig) {
		cfg.Features.LogLevel = "debug"
		cfg.Server.Port = 9090
	})
	if updateErr != nil {
		log.Printf("更新配置失败: %v", updateErr)
	} else {
		// 获取更新后的配置
		updatedCfg, err := manager.GetConfig()
		if err == nil {
			fmt.Printf("更新后配置: Port=%d, LogLevel=%s\n",
				updatedCfg.Server.Port, updatedCfg.Features.LogLevel)
		}
	}

	// 7. 获取拦截统计信息
	fmt.Println("\n--- 拦截统计信息 ---")
	if manager.IsRefactoredMode() {
		refactoredManager := manager.GetRefactoredManager()
		if refactoredManager != nil {
			stats := refactoredManager.GetInterceptionStats()
			for key, value := range stats {
				fmt.Printf("  %s: %v\n", key, value)
			}
		} else {
			fmt.Println("重构管理器不可用，无法获取拦截统计")
		}
	} else {
		fmt.Println("未启用重构模式，无拦截统计")
	}

	// 8. 优雅关闭
	fmt.Println("\n--- 优雅关闭 ---")
	if err := manager.Close(5 * time.Second); err != nil {
		log.Printf("关闭管理器失败: %v", err)
	} else {
		fmt.Println("管理器已成功关闭")
	}

	fmt.Println("\n=== 示例完成 ===")
}

// main 函数调用simpleMain来避免与其他示例的main函数冲突
// 这个文件演示了简单的集成配置管理器使用方式
// 如果要运行这个示例，请将此文件重命名为main.go并删除其他main.go文件
func init() {
	// 这个函数在包初始化时会被调用，但不会与main函数冲突
	// 如果需要运行这个示例，请手动调用simpleMain()
}
