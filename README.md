# go-config

`go-config` 是一个基于 [Viper](https://github.com/spf13/viper) 的 Go 语言泛型配置管理库。它提供了类型安全的配置加载、自动监听文件变更（热重载）、防抖处理以及灵活的 Hook 系统。

## 特性

- **泛型支持**：利用 Go 泛型提供类型安全的配置访问
- **热重载**：自动监听配置文件变更并实时更新应用配置
- **防抖处理**：内置防抖机制，防止因文件系统高频触发导致的多次重复加载
- **Hook 系统**：支持在初始化、调试、信息、警告和错误等阶段注入自定义逻辑
- **动态更新**：支持通过代码动态修改配置字段，并自动同步回配置文件
- **灵活配置**：支持自定义配置文件名、路径及监控间隔
- **线程安全**：完全线程安全的配置访问和更新
- **Goroutine 管理**：统一的 goroutine 调度，避免资源泄漏
- **配置版本管理**：支持配置快照、版本控制和回滚
- **优雅关闭**：支持优雅关闭和资源清理

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
        Debug   bool   `yaml:"debug" mapstructure:"debug"`
    } `yaml:"app" mapstructure:"app"`
    
    Database struct {
        Host     string `yaml:"host" mapstructure:"host"`
        Port     int    `yaml:"port" mapstructure:"port"`
        Username string `yaml:"username" mapstructure:"username"`
        Password string `yaml:"password" mapstructure:"password"`
    } `yaml:"database" mapstructure:"database"`
    
    Server struct {
        Host string `yaml:"host" mapstructure:"host"`
        Port int    `yaml:"port" mapstructure:"port"`
    } `yaml:"server" mapstructure:"server"`
}

// 实现 Configurable 接口
func (c *AppConfig) Validate() error {
    if c.App.Name == "" {
        return fmt.Errorf("app name cannot be empty")
    }
    if c.Server.Port <= 0 || c.Server.Port > 65535 {
        return fmt.Errorf("invalid server port: %d", c.Server.Port)
    }
    return nil
}
```

### 2. 基础使用示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/rei0721/go-config"
)

func main() {
    // 1. 定义默认配置
    defaultConfig := &AppConfig{
        App: struct {
            Name    string `yaml:"name" mapstructure:"name"`
            Version string `yaml:"version" mapstructure:"version"`
            Debug   bool   `yaml:"debug" mapstructure:"debug"`
        }{
            Name:    "MyApp",
            Version: "1.0.0",
            Debug:   false,
        },
        Server: struct {
            Host string `yaml:"host" mapstructure:"host"`
            Port int    `yaml:"port" mapstructure:"port"`
        }{
            Host: "localhost",
            Port: 8080,
        },
    }

    // 2. 创建配置管理器
    manager := config.NewRefactoredManager(defaultConfig)

    // 3. 构建配置选项
    option, err := config.NewOptionBuilder().
        WithFilename("config.yaml").
        WithFilepath("./configs").
        WithDebounceDuration(500 * time.Millisecond).
        WithMaxWorkers(10).
        WithBufferSize(100).
        Build()
    if err != nil {
        log.Fatal("Failed to build options:", err)
    }

    // 4. 初始化管理器
    ctx := context.Background()
    if err := manager.Initialize(ctx, option); err != nil {
        log.Fatal("Failed to initialize manager:", err)
    }
    defer manager.Close(5 * time.Second)

    // 5. 加载配置
    if err := manager.Load(ctx); err != nil {
        log.Fatal("Failed to load config:", err)
    }

    // 6. 获取配置
    cfg, err := manager.GetConfig()
    if err != nil {
        log.Fatal("Failed to get config:", err)
    }

    fmt.Printf("App: %s v%s\n", cfg.App.Name, cfg.App.Version)
    fmt.Printf("Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)

    // 7. 启动文件监听（可选）
    if err := manager.StartWatching(ctx); err != nil {
        log.Printf("Failed to start watching: %v", err)
    }

    // 保持程序运行
    select {}
}
```

### 3. 高级使用示例

#### 3.1 使用 Hook 系统

```go
// 注册各种类型的钩子
func setupHooks(manager *config.RefactoredManager[AppConfig]) {
    // 初始化钩子
    manager.RegisterHook(config.HookTypeInit, &InitHook{})
    
    // 配置加载前钩子
    manager.RegisterHook(config.HookTypeBeforeLoad, &ValidationHook{})
    
    // 配置加载后钩子
    manager.RegisterHook(config.HookTypeAfterLoad, &NotificationHook{})
    
    // 错误处理钩子
    manager.RegisterHook(config.HookTypeError, &ErrorLogHook{})
}

// 自定义钩子实现
type InitHook struct{}

func (h *InitHook) Handle(ctx *config.HookContext) error {
    log.Printf("配置管理器初始化完成: %s", ctx.Message)
    return nil
}

func (h *InitHook) Priority() int { return 0 }
func (h *InitHook) IsAsync() bool { return false }

type ValidationHook struct{}

func (h *ValidationHook) Handle(ctx *config.HookContext) error {
    log.Printf("开始加载配置: %s", ctx.Message)
    // 执行预加载验证逻辑
    return nil
}

func (h *ValidationHook) Priority() int { return 1 }
func (h *ValidationHook) IsAsync() bool { return false }

type NotificationHook struct{}

func (h *NotificationHook) Handle(ctx *config.HookContext) error {
    if cfg, ok := ctx.Config.(*AppConfig); ok {
        log.Printf("配置加载完成: %s v%s", cfg.App.Name, cfg.App.Version)
        // 发送通知到其他系统
    }
    return nil
}

func (h *NotificationHook) Priority() int { return 0 }
func (h *NotificationHook) IsAsync() bool { return true }

type ErrorLogHook struct{}

func (h *ErrorLogHook) Handle(ctx *config.HookContext) error {
    log.Printf("配置管理器错误: %v", ctx.Error)
    // 发送错误报告
    return nil
}

func (h *ErrorLogHook) Priority() int { return 0 }
func (h *ErrorLogHook) IsAsync() bool { return true }
```

#### 3.2 配置变更处理

```go
func main() {
    manager := config.NewRefactoredManager(defaultConfig)
    
    // 初始化...
    
    // 注册配置变更处理器
    changeHandler := func(ctx context.Context, oldConfig, newConfig *AppConfig) error {
        log.Printf("配置发生变更:")
        log.Printf("  旧版本: %s", oldConfig.App.Version)
        log.Printf("  新版本: %s", newConfig.App.Version)
        
        // 处理配置变更逻辑
        if oldConfig.Server.Port != newConfig.Server.Port {
            log.Printf("服务器端口从 %d 变更为 %d", oldConfig.Server.Port, newConfig.Server.Port)
            // 重启服务器等操作
        }
        
        return nil
    }
    
    // 加载配置并注册变更处理器
    if err := manager.Load(ctx, changeHandler); err != nil {
        log.Fatal(err)
    }
    
    // 启动监听
    if err := manager.StartWatching(ctx); err != nil {
        log.Fatal(err)
    }
}
```

#### 3.3 动态配置更新

```go
func updateConfig(manager *config.RefactoredManager[AppConfig]) {
    // 动态更新配置
    err := manager.UpdateConfig(context.Background(), func(cfg *AppConfig) {
        cfg.App.Debug = true
        cfg.Server.Port = 9090
    })
    if err != nil {
        log.Printf("更新配置失败: %v", err)
        return
    }
    
    log.Println("配置更新成功")
}
```

#### 3.4 配置快照和版本管理

```go
func manageConfigVersions(manager *config.RefactoredManager[AppConfig]) {
    store := manager.GetConfigStore()
    
    // 创建快照
    version, err := store.CreateSnapshot("手动备份")
    if err != nil {
        log.Printf("创建快照失败: %v", err)
        return
    }
    log.Printf("创建快照成功，版本: %d", version)
    
    // 列出所有快照
    versions := store.ListSnapshots()
    log.Printf("可用快照版本: %v", versions)
    
    // 获取快照详情
    snapshot, err := store.GetSnapshot(version)
    if err != nil {
        log.Printf("获取快照失败: %v", err)
        return
    }
    log.Printf("快照信息: 版本=%d, 时间=%v, 描述=%s", 
        snapshot.Version, snapshot.Timestamp, snapshot.Description)
    
    // 从快照恢复配置
    if err := store.RestoreFromSnapshot(version); err != nil {
        log.Printf("恢复快照失败: %v", err)
        return
    }
    log.Println("配置恢复成功")
}
```

## API 参考

### 核心接口

#### RefactoredConfigManager[T]

主要的配置管理器接口，提供完整的配置管理功能。

```go
type RefactoredConfigManager[T Configurable] interface {
    Initialize(ctx context.Context, option *ImmutableOption) error
    Load(ctx context.Context, handlers ...ChangeHandler[T]) error
    GetConfig() (*T, error)
    UpdateConfig(ctx context.Context, updateFunc func(*T)) error
    RegisterHook(hookType HookType, handler HookHandler) error
    StartWatching(ctx context.Context) error
    StopWatching(timeout time.Duration) error
    Close(timeout time.Duration) error
    GetStatus() *ManagerStatus
}
```

#### OptionBuilder

配置选项构建器，用于创建配置管理器的选项。

```go
type OptionBuilder struct {
    // 私有字段
}

func NewOptionBuilder() *OptionBuilder
func (b *OptionBuilder) WithFilename(filename string) *OptionBuilder
func (b *OptionBuilder) WithFilepath(filepath string) *OptionBuilder
func (b *OptionBuilder) WithDebounceDuration(dur time.Duration) *OptionBuilder
func (b *OptionBuilder) WithMaxWorkers(max int) *OptionBuilder
func (b *OptionBuilder) WithBufferSize(size int) *OptionBuilder
func (b *OptionBuilder) Build() (*ImmutableOption, error)
```

### Hook 系统

#### HookHandler 接口

```go
type HookHandler interface {
    Handle(ctx *HookContext) error
    Priority() int
    IsAsync() bool
}
```

#### HookType 枚举

```go
const (
    HookTypeInit HookType = iota
    HookTypeBeforeLoad
    HookTypeAfterLoad
    HookTypeBeforeUpdate
    HookTypeAfterUpdate
    HookTypeError
    HookTypeDebug
    HookTypeInfo
    HookTypeWarn
)
```

### 配置存储

#### ConfigStore[T]

线程安全的配置存储，支持版本管理和快照。

```go
type ConfigStore[T Configurable] struct {
    // 私有字段
}

func (cs *ConfigStore[T]) GetConfig() (*T, error)
func (cs *ConfigStore[T]) SetConfig(newConfig *T) error
func (cs *ConfigStore[T]) UpdateConfig(updateFunc func(*T) (*T, error)) error
func (cs *ConfigStore[T]) CreateSnapshot(description string) (int64, error)
func (cs *ConfigStore[T]) RestoreFromSnapshot(version int64) error
```

## 最佳实践

### 1. 配置结构设计

```go
// 推荐：使用嵌套结构组织配置
type Config struct {
    // 应用基础配置
    App AppConfig `yaml:"app" mapstructure:"app"`
    
    // 数据库配置
    Database DatabaseConfig `yaml:"database" mapstructure:"database"`
    
    // 服务器配置
    Server ServerConfig `yaml:"server" mapstructure:"server"`
    
    // 日志配置
    Logging LoggingConfig `yaml:"logging" mapstructure:"logging"`
}

// 为每个子配置提供验证方法
func (c *Config) Validate() error {
    if err := c.App.Validate(); err != nil {
        return fmt.Errorf("app config: %w", err)
    }
    if err := c.Database.Validate(); err != nil {
        return fmt.Errorf("database config: %w", err)
    }
    return nil
}
```

### 2. 错误处理

```go
// 推荐：使用专门的错误处理钩子
type ErrorHandler struct {
    logger *log.Logger
}

func (h *ErrorHandler) Handle(ctx *config.HookContext) error {
    h.logger.Printf("配置错误: %v", ctx.Error)
    
    // 根据错误类型采取不同的处理策略
    switch ctx.Error.(type) {
    case *os.PathError:
        // 文件路径错误，尝试创建默认配置
        return h.createDefaultConfig()
    case *yaml.TypeError:
        // YAML 格式错误，记录详细信息
        return h.logYAMLError(ctx.Error)
    default:
        // 其他错误，发送告警
        return h.sendAlert(ctx.Error)
    }
}
```

### 3. 性能优化

```go
// 推荐：合理设置调度池参数
option, err := config.NewOptionBuilder().
    WithMaxWorkers(runtime.NumCPU() * 2).  // 基于 CPU 核心数
    WithBufferSize(200).                   // 缓冲区大小为工作器的 10 倍
    WithDebounceDuration(300 * time.Millisecond).  // 适中的防抖时间
    Build()
```

### 4. 优雅关闭

```go
func main() {
    manager := config.NewRefactoredManager(defaultConfig)
    
    // 设置信号处理
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-c
        log.Println("正在关闭配置管理器...")
        
        // 优雅关闭，等待最多 10 秒
        if err := manager.Close(10 * time.Second); err != nil {
            log.Printf("关闭配置管理器失败: %v", err)
        }
        
        os.Exit(0)
    }()
    
    // 应用主逻辑...
}
```

### 5. 监控和调试

```go
// 推荐：定期检查管理器状态
func monitorManager(manager *config.RefactoredManager[AppConfig]) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        status := manager.GetStatus()
        log.Printf("配置管理器状态: 初始化=%v, 监听=%v, 版本=%d, 错误数=%d",
            status.Initialized, status.Watching, status.ConfigVersion, status.ErrorCount)
        
        // 检查调度池状态
        if pool := manager.GetSchedulerPool(); pool != nil {
            metrics := pool.GetDetailedMetrics()
            log.Printf("调度池状态: 活跃工作器=%d, 队列任务=%d, 成功率=%.2f",
                metrics.ActiveWorkers, metrics.QueuedTasks, metrics.SuccessRate)
        }
    }
}
```

## 故障排除

### 常见问题

1. **配置文件不存在**
   - 管理器会自动创建默认配置文件
   - 确保目录有写入权限

2. **文件监听失败**
   - 检查文件路径是否正确
   - 确保文件系统支持 inotify（Linux）或类似机制

3. **内存使用过高**
   - 减少快照保留数量
   - 调整调度池大小
   - 检查是否有内存泄漏

4. **配置更新不生效**
   - 检查文件格式是否正确
   - 确认防抖时间设置
   - 查看错误日志

### 调试技巧

```go
// 启用详细日志
manager.RegisterHook(config.HookTypeDebug, &DebugHook{})
manager.RegisterHook(config.HookTypeInfo, &InfoHook{})

// 获取详细状态信息
stats := manager.GetInterceptionStats()
log.Printf("拦截统计: %+v", stats)
```

## 许可证

[MIT License](LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request！

## 更新日志

### v1.0.0
- 重构架构，引入调度池和钩子系统
- 添加配置版本管理和快照功能
- 改进线程安全性和性能
- 支持优雅关闭和资源管理

### v0.1.0
- 初始版本
- 基础配置管理功能
- 文件监听和热重载
