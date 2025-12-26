package main

import (
	"fmt"
	"time"

	"github.com/rei0721/go-config"
)

type Logger struct {
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) log(level, format string, field ...string) {
	fmt.Printf("[LOG] [%s] %s\n", level, format)
}

func (l *Logger) Debug(format string, field ...string) {
	l.log("DEBUG", format)
}

func (l *Logger) Info(format string, field ...string) {
	l.log("INFO", format)
}

func (l *Logger) Warn(format string, field ...string) {
	l.log("WARN", format)
}

func (l *Logger) Error(format string, field ...string) {
	l.log("ERROR", format)
}

// App 应用基础配置
type App struct {
	Name        string `yaml:"name" mapstructure:"name"`
	Version     string `yaml:"version" mapstructure:"version"`
	Description string `yaml:"description" mapstructure:"description"`
}

type Configs struct {
	App App `yaml:"app" mapstructure:"app"`
}

func main() {
	// 实例化日志
	log := NewLogger()

	// 设置配置选项
	opts := config.NewOption()
	opts.Filename.Set("config.dev.yaml")                     // production | development
	opts.Filepath.Set("./configs")                           // 设置文件夹
	opts.DebounceDur.Set(800 * config.OptionDateMillisecond) // 设置防抖

	// 实例化配置管理器
	manager := config.NewManager[Configs](&Configs{
		App: App{
			Name:        "qwq",
			Version:     "1.0.0",
			Description: "QWQ App.",
		},
	})

	// setting config hook
	manager.SetHook(config.InitHook, func(ctx config.HookContext) {
		log.Info("正在初始化配置...")
	}).SetHook(config.Debug, func(ctx config.HookContext) {
		log.Debug(ctx.Message)
	}).SetHook(config.Info, func(ctx config.HookContext) {
		log.Info(ctx.Message)
	}).SetHook(config.Warn, func(ctx config.HookContext) {
		log.Warn(ctx.Message)
	}).SetHook(config.Error, func(ctx config.HookContext) {
		log.Error(ctx.Message)
	})

	manager.SetOption(opts)

	if err := manager.Load(func(ctx *config.Context) {
		config := ctx.Config.(*Configs)
		log.Debug("配置文件更新了, app: " + config.App.Name)
	}); err != nil {
		log.Error(fmt.Sprintf("初始化配置失败 Error: %s", err.Error()))
	}

	log.Info("初始化成功")

	go func(manager *config.Manager[Configs]) {
		time.Sleep(time.Second * 2)
		manager.UpdateField(func(config *Configs) {
			config.App.Name = "config"
		})
	}(manager)

	select {}
}
