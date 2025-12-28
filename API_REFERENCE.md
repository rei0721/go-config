# go-config API 参考文档

本文档提供了 go-config 库的完整 API 参考，包括所有公共接口、类型定义和使用示例。

## 目录

- [核心接口](#核心接口)
- [配置选项](#配置选项)
- [钩子系统](#钩子系统)
- [调度池](#调度池)
- [文件监听](#文件监听)
- [配置存储](#配置存储)
- [错误处理](#错误处理)
- [类型定义](#类型定义)

## 核心接口

### RefactoredConfigManager[T]

重构后的配置管理器主接口，提供完整的配置管理功能。

```go
type RefactoredConfigManager[T Configurable] interface {
    // 初始化配置管理器
    Initialize(ctx context.Context, option *ImmutableOption) error
    
    // 加载配置
    Load(ctx context.Context, handlers ...ChangeHandler[T]) error
    
    // 获取配置
    GetConfig() (*T, error)
    
    // 更新配置
    UpdateConfig(ctx context.Context, updateFunc func(*T)) error
    
    // 注册钩子
    RegisterHook(hookType HookType, handler HookHandler) error
    
    // 启动监听
    StartWatching(ctx context.Context) error
    
    // 停止监听
    StopWatching(timeout time.Duration) error
    
    // 关闭管理器
    Close(timeout time.Duration) error
    
    // 获取状态
    GetStatus() *ManagerStatus
}
```

#### 方法详解

##### Initialize

初始化配置管理器，设置所有必要的组件。

```go
func (m *RefactoredManager[T]) Initialize(ctx context.Context, option *ImmutableOption) error
```

**参数：**
- `ctx`: 上下文，用于控制初始化过程
- `option`: 不可变配置选项，包含所有配置参数

**返回值：**
- `error`: 初始化失败时的错误信息

**使用示例：**
```go
option, _ := config.NewOptionBuilder().
    WithFilename("config.yaml").
    WithFilepath("./configs").
    Build()

err := manager.Initialize(context.Background(), option)
if err != nil {
    log.Fatal("初始化失败:", err)
}
```

##### Load

加载配置文件并可选地注册变更处理器。

```go
func (m *RefactoredManager[T]) Load(ctx context.Context, handlers ...ChangeHandler[T]) error
```

**参数：**
- `ctx`: 上下文
- `handlers`: 可选的配置变更处理器

**返回值：**
- `error`: 加载失败时的错误信息

**使用示例：**
```go
changeHandler := func(ctx context.Context, oldConfig, newConfig *AppConfig) error {
    log.Printf("配置从版本 %s 更新到 %s", oldConfig.App.Version, newConfig.App.Version)
    return nil
}

err := manager.Load(context.Background(), changeHandler)
```

##### GetConfig

获取当前配置的副本。

```go
func (m *RefactoredManager[T]) GetConfig() (*T, error)
```

**返回值：**
- `*T`: 配置对象的副本
- `error`: 获取失败时的错误信息

**使用示例：**
```go
config, err := manager.GetConfig()
if err != nil {
    log.Fatal("获取配置失败:", err)
}
fmt.Printf("当前配置: %+v", config)
```

##### UpdateConfig

动态更新配置。

```go
func (m *RefactoredManager[T]) UpdateConfig(ctx context.Context, updateFunc func(*T)) error
```

**参数：**
- `ctx`: 上下文
- `updateFunc`: 配置更新函数

**返回值：**
- `error`: 更新失败时的错误信息

**使用示例：**
```go
err := manager.UpdateConfig(context.Background(), func(cfg *AppConfig) {
    cfg.App.Debug = true
    cfg.Server.Port = 9090
})
```

### NewRefactoredManager

创建新的配置管理器实例。

```go
func NewRefactoredManager[T Configurable](defaultConfig *T) *RefactoredManager[T]
```

**参数：**
- `defaultConfig`: 默认配置对象，不能为 nil

**返回值：**
- `*RefactoredManager[T]`: 新的配置管理器实例

**使用示例：**
```go
defaultConfig := &AppConfig{
    App: AppInfo{Name: "MyApp", Version: "1.0.0"},
    Server: ServerConfig{Host: "localhost", Port: 8080},
}

manager := config.NewRefactoredManager(defaultConfig)
```

## 配置选项

### OptionBuilder

配置选项构建器，提供链式调用 API。

```go
type OptionBuilder struct {
    // 私有字段
}
```

#### 方法

##### NewOptionBuilder

创建新的配置选项构建器。

```go
func NewOptionBuilder() *OptionBuilder
```

##### WithFilename

设置配置文件名。

```go
func (b *OptionBuilder) WithFilename(filename string) *OptionBuilder
```

**参数：**
- `filename`: 配置文件名，必须包含有效扩展名（.yaml, .yml, .json, .toml）

**验证规则：**
- 不能为空
- 不能包含路径分隔符
- 必须有支持的文件扩展名

##### WithFilepath

设置配置文件路径。

```go
func (b *OptionBuilder) WithFilepath(filepath string) *OptionBuilder
```

**参数：**
- `filepath`: 配置文件目录路径

##### WithDebounceDuration

设置防抖时间。

```go
func (b *OptionBuilder) WithDebounceDuration(dur time.Duration) *OptionBuilder
```

**参数：**
- `dur`: 防抖时间间隔，建议 10ms-10s

##### WithMaxWorkers

设置最大工作 goroutine 数量。

```go
func (b *OptionBuilder) WithMaxWorkers(max int) *OptionBuilder
```

**参数：**
- `max`: 最大工作器数量，建议 1-1000

##### WithBufferSize

设置任务队列缓冲区大小。

```go
func (b *OptionBuilder) WithBufferSize(size int) *OptionBuilder
```

**参数：**
- `size`: 缓冲区大小，建议是 maxWorkers 的 2-10 倍

##### Build

构建不可变的配置选项。

```go
func (b *OptionBuilder) Build() (*ImmutableOption, error)
```

**返回值：**
- `*ImmutableOption`: 不可变配置选项
- `error`: 构建失败时的错误信息

### ImmutableOption

不可变的配置选项对象。

```go
type ImmutableOption struct {
    // 私有字段
}
```

#### 方法

所有方法都是只读的，返回配置值：

```go
func (o *ImmutableOption) GetFilename() string
func (o *ImmutableOption) GetFilepath() string
func (o *ImmutableOption) GetDebounceDuration() time.Duration
func (o *ImmutableOption) GetMaxWorkers() int
func (o *ImmutableOption) GetBufferSize() int
func (o *ImmutableOption) GetLoadTimeout() time.Duration
func (o *ImmutableOption) GetUpdateTimeout() time.Duration
func (o *ImmutableOption) GetShutdownTimeout() time.Duration
func (o *ImmutableOption) GetFullPath() string
```

## 钩子系统

### HookHandler

钩子处理器接口。

```go
type HookHandler interface {
    // 处理钩子事件
    Handle(ctx *HookContext) error
    
    // 返回处理器优先级，数值越小优先级越高
    Priority() int
    
    // 返回是否异步执行
    IsAsync() bool
}
```

### HookContext

钩子执行上下文。

```go
type HookContext struct {
    Type      HookType               // 钩子类型
    Message   string                 // 消息内容
    Config    interface{}            // 配置对象
    Error     error                  // 错误信息（如果有）
    Metadata  map[string]interface{} // 元数据
    Timestamp time.Time              // 时间戳
}
```

### HookType

钩子类型枚举。

```go
type HookType int

const (
    HookTypeInit HookType = iota        // 初始化时触发
    HookTypeBeforeLoad                  // 配置加载前触发
    HookTypeAfterLoad                   // 配置加载后触发
    HookTypeBeforeUpdate                // 配置更新前触发
    HookTypeAfterUpdate                 // 配置更新后触发
    HookTypeError                       // 发生错误时触发
    HookTypeDebug                       // 调试信息时触发
    HookTypeInfo                        // 一般信息时触发
    HookTypeWarn                        // 警告信息时触发
)
```

### HookRegistry

钩子注册表，管理所有钩子处理器。

```go
type HookRegistry struct {
    // 私有字段
}
```

#### 方法

```go
func NewHookRegistry() *HookRegistry
func (r *HookRegistry) RegisterHook(hookType HookType, handler HookHandler) error
func (r *HookRegistry) UnregisterHook(hookType HookType, handler HookHandler) error
func (r *HookRegistry) GetHandlers(hookType HookType) []HookHandler
func (r *HookRegistry) Clear(hookType HookType)
func (r *HookRegistry) ClearAll()
```

## 调度池

### SchedulerPool

goroutine 调度池，管理所有异步任务。

```go
type SchedulerPool struct {
    // 私有字段
}
```

#### 方法

##### NewSchedulerPool

创建新的调度池。

```go
func NewSchedulerPool(maxWorkers, bufferSize int) *SchedulerPool
```

##### Start

启动调度池。

```go
func (p *SchedulerPool) Start(ctx context.Context) error
```

##### Stop

停止调度池。

```go
func (p *SchedulerPool) Stop(timeout time.Duration) error
```

##### Submit

提交任务。

```go
func (p *SchedulerPool) Submit(task Task) error
```

##### SubmitFunc

提交函数任务。

```go
func (p *SchedulerPool) SubmitFunc(fn func(context.Context) error) error
```

### Task

任务接口。

```go
type Task interface {
    Execute(ctx context.Context) error
    Priority() int
    ID() string
}
```

### PoolMetrics

调度池统计信息。

```go
type PoolMetrics struct {
    ActiveWorkers   int32
    QueuedTasks     int32
    CompletedTasks  int64
    FailedTasks     int64
    // ... 其他字段
}
```

## 文件监听

### FileWatcher

文件变更监听器。

```go
type FileWatcher struct {
    // 私有字段
}
```

#### 方法

```go
func NewFileWatcher(filepath string, debouncer *Debouncer, pool *SchedulerPool) *FileWatcher
func (w *FileWatcher) Start(ctx context.Context) error
func (w *FileWatcher) Stop(timeout time.Duration) error
func (w *FileWatcher) AddHandler(handler FileChangeHandler)
func (w *FileWatcher) AddHandlerWithID(handler *FileChangeHandlerWithID) error
func (w *FileWatcher) IsRunning() bool
func (w *FileWatcher) GetStatus() FileWatcherStatus
```

### FileChangeHandler

文件变更处理器函数类型。

```go
type FileChangeHandler func(event fsnotify.Event) error
```

### Debouncer

防抖器。

```go
type Debouncer struct {
    // 私有字段
}

func NewDebouncer(duration time.Duration) *Debouncer
func (d *Debouncer) Debounce(fn func())
```

## 配置存储

### ConfigStore[T]

线程安全的配置存储。

```go
type ConfigStore[T Configurable] struct {
    // 私有字段
}
```

#### 方法

```go
func NewConfigStore[T Configurable](defaultConfig *T, maxSnapshots int) *ConfigStore[T]
func (cs *ConfigStore[T]) GetConfig() (*T, error)
func (cs *ConfigStore[T]) SetConfig(newConfig *T) error
func (cs *ConfigStore[T]) UpdateConfig(updateFunc func(*T) (*T, error)) error
func (cs *ConfigStore[T]) CreateSnapshot(description string) (int64, error)
func (cs *ConfigStore[T]) GetSnapshot(version int64) (*ConfigSnapshot[T], error)
func (cs *ConfigStore[T]) RestoreFromSnapshot(version int64) error
func (cs *ConfigStore[T]) ListSnapshots() []int64
```

### ConfigSnapshot[T]

配置快照。

```go
type ConfigSnapshot[T Configurable] struct {
    Config      *T        // 快照的配置数据
    Version     int64     // 快照版本号
    Timestamp   time.Time // 快照创建时间
    Description string    // 快照描述信息
}
```

## 错误处理

### 错误类型

库定义了以下错误常量：

```go
var (
    ErrInvalidHandler           = errors.New("invalid handler")
    ErrHandlerAlreadyRegistered = errors.New("handler already registered")
    ErrHandlerNotFound          = errors.New("handler not found")
    ErrInvalidTask              = errors.New("invalid task")
    ErrTaskQueueFull            = errors.New("task queue full")
    ErrPoolNotRunning           = errors.New("pool not running")
    ErrWatcherNotRunning        = errors.New("watcher not running")
    ErrWatcherStopTimeout       = errors.New("watcher stop timeout")
    ErrInvalidFilepath          = errors.New("invalid filepath")
    ErrInvalidFileHandler       = errors.New("invalid file handler")
)
```

## 类型定义

### Configurable

配置对象必须实现的接口。

```go
type Configurable interface {
    Validate() error
}
```

### ChangeHandler[T]

配置变更处理器函数类型。

```go
type ChangeHandler[T Configurable] func(ctx context.Context, oldConfig, newConfig *T) error
```

### ManagerStatus

管理器状态信息。

```go
type ManagerStatus struct {
    Initialized   bool          // 是否已初始化
    Watching      bool          // 是否正在监听
    LastUpdate    time.Time     // 最后更新时间
    ConfigVersion int64         // 配置版本
    ErrorCount    int64         // 错误计数
    LastError     error         // 最后错误
    Uptime        time.Duration // 运行时间
    StartTime     time.Time     // 启动时间
}
```

## 使用示例

### 完整示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/rei0721/go-config"
)

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
}

func (c *AppConfig) Validate() error {
    if c.App.Name == "" {
        return fmt.Errorf("app name is required")
    }
    if c.Server.Port <= 0 {
        return fmt.Errorf("invalid server port")
    }
    return nil
}

func main() {
    // 1. 创建默认配置
    defaultConfig := &AppConfig{
        App: struct {
            Name    string `yaml:"name"`
            Version string `yaml:"version"`
            Debug   bool   `yaml:"debug"`
        }{
            Name:    "MyApp",
            Version: "1.0.0",
            Debug:   false,
        },
        Server: struct {
            Host string `yaml:"host"`
            Port int    `yaml:"port"`
        }{
            Host: "localhost",
            Port: 8080,
        },
    }

    // 2. 创建管理器
    manager := config.NewRefactoredManager(defaultConfig)

    // 3. 构建选项
    option, err := config.NewOptionBuilder().
        WithFilename("app.yaml").
        WithFilepath("./config").
        WithDebounceDuration(500 * time.Millisecond).
        WithMaxWorkers(5).
        WithBufferSize(50).
        Build()
    if err != nil {
        log.Fatal("构建选项失败:", err)
    }

    // 4. 初始化
    ctx := context.Background()
    if err := manager.Initialize(ctx, option); err != nil {
        log.Fatal("初始化失败:", err)
    }
    defer manager.Close(5 * time.Second)

    // 5. 注册钩子
    manager.RegisterHook(config.HookTypeAfterLoad, &LogHook{})

    // 6. 加载配置
    changeHandler := func(ctx context.Context, old, new *AppConfig) error {
        log.Printf("配置变更: %s -> %s", old.App.Version, new.App.Version)
        return nil
    }

    if err := manager.Load(ctx, changeHandler); err != nil {
        log.Fatal("加载配置失败:", err)
    }

    // 7. 启动监听
    if err := manager.StartWatching(ctx); err != nil {
        log.Printf("启动监听失败: %v", err)
    }

    // 8. 使用配置
    cfg, _ := manager.GetConfig()
    fmt.Printf("应用: %s v%s, 服务器: %s:%d\n",
        cfg.App.Name, cfg.App.Version, cfg.Server.Host, cfg.Server.Port)

    // 保持运行
    select {}
}

type LogHook struct{}

func (h *LogHook) Handle(ctx *config.HookContext) error {
    log.Printf("钩子触发: %s - %s", ctx.Type, ctx.Message)
    return nil
}

func (h *LogHook) Priority() int { return 0 }
func (h *LogHook) IsAsync() bool { return false }
```

这个 API 参考文档涵盖了 go-config 库的所有主要接口和功能。每个接口都包含了详细的参数说明、返回值描述和使用示例。