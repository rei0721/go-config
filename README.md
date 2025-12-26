# go-config

`go-config` 是一个基于 [Viper](https://github.com/spf13/viper) 的 Go 语言泛型配置管理库。它提供了类型安全的配置加载、自动监听文件变更（热重载）、防抖处理以及灵活的 Hook 系统。

## 特性

- **泛型支持**：利用 Go 泛型提供类型安全的配置访问。
- **热重载**：自动监听配置文件变更并实时更新应用配置。
- **防抖处理**：内置防抖机制，防止因文件系统高频触发导致的多次重复加载。
- **Hook 系统**：支持在初始化、调试、信息、警告和错误等阶段注入自定义逻辑。
- **动态更新**：支持通过代码动态修改配置字段，并自动同步回配置文件。
- **灵活配置**：支持自定义配置文件名、路径及监控间隔。

## 安装

```bash
go get github.com/rei0721/go-config
```

## 快速开始

### 1. 定义配置结构体

```go
type AppConfig struct {
    App struct {
        Name    string `yaml:"name" mapstructure:"name"`
        Version string `yaml:"version" mapstructure:"version"`
    } `yaml:"app" mapstructure:"app"`
}
```

### 2. 初始化管理器

```go
package main

import (
    "fmt"
    "github.com/rei0721/go-config"
)

func main() {
    // 1. 设置配置选项
    opts := config.NewOption()
    opts.Filename.Set("config.yaml")
    opts.Filepath.Set("./configs")

    // 2. 实例化配置管理器
    manager := config.NewManager[AppConfig](&AppConfig{
        // 默认值
    })

    // 3. 设置 Hook（可选）
    manager.SetHook(config.Info, func(ctx config.HookContext) {
        fmt.Println("[INFO]", ctx.Message)
    })

    manager.SetOption(opts)

    // 4. 加载配置并启动监听
    if err := manager.Load(func(ctx *config.Context) {
        cfg := ctx.Config.(*AppConfig)
        fmt.Printf("配置已更新: %+v\n", cfg.App)
    }); err != nil {
        panic(err)
    }

    // 获取当前配置
    cfg, _ := manager.GetConfig()
    fmt.Println("当前应用名:", cfg.App.Name)

    select {}
}
```

## 核心 API 说明

### 配置选项 (Option)

通过 `config.NewOption()` 创建，支持以下设置：

- `Filename`: 配置文件名（默认为 `config.yaml`）。
- `Filepath`: 配置文件所在目录（默认为 `./configs`）。
- `DebounceDur`: 文件变更监控的防抖时间间隔（默认为 `800ms`）。

### Hook 系统

支持以下几种 Hook 模式：

- `config.InitHook`: 初始化时触发。
- `config.Debug`: 调试信息触发。
- `config.Info`: 普通信息触发。
- `config.Warn`: 警告信息触发。
- `config.Error`: 错误信息触发。

### 动态更新

可以使用 `UpdateField` 方法安全地修改配置：

```go
manager.UpdateField(func(cfg *AppConfig) {
    cfg.App.Name = "NewAppName"
})
```

此操作会更新内存中的配置，并尝试将更改写回配置文件。

## 许可证

[MIT License](LICENSE)
