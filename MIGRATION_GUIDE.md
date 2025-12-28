# 配置管理库迁移指南

## 概述

本指南帮助您从配置管理库的旧版本（1.x）迁移到新版本（2.0.0）。新版本提供了重构后的架构，包含更好的线程安全、goroutine管理、以及高级功能。

## 版本兼容性

- **当前版本**: 2.0.0
- **支持的旧版本**: 1.0.0+
- **向后兼容**: ✅ 是
- **自动迁移**: ⚠️ 部分支持

## 主要变更

### 新增功能

1. **重构架构**: 全新的组件化架构
2. **调度池**: 统一的goroutine管理
3. **线程安全存储**: 改进的配置存储机制
4. **高级文件监听**: 可停止的文件监听器
5. **增强钩子系统**: 更灵活的事件处理
6. **Viper拦截**: 防止goroutine泄露
7. **属性测试支持**: 内置的正确性验证

### 废弃功能

1. `Manager.UpdateField` → `Manager.UpdateConfigSafe`
2. `Manager.GetConfig` → `Manager.GetConfigSafe`
3. 旧的Hook API → 新的HookHandler接口
4. 旧的Option API → 新的OptionBuilder模式

### 破坏性变更

1. Manager结构体字段现在是私有的
2. Hook系统API完全重构
3. Option系统完全重构

## 迁移步骤

### 步骤 1: 更新导入

```go
// 确保使用正确的包导入路径
import "github.com/rei0721/go-config"
```

### 步骤 2: 使用兼容性层（推荐）

最简单的迁移方式是使用兼容性层：

```go
// 旧代码
manager := config.NewManager(defaultConfig)

// 新代码（兼容性层）
manager := config.Default(defaultConfig)
```

### 步骤 3: 逐步启用高级功能

```go
// 启用高级功能
manager := config.Default(defaultConfig)
manager.EnableAdvancedFeatures()

// 现在可以使用新功能
err := manager.StartWatching()
advancedManager := manager.GetAdvancedManager()
```

### 步骤 4: 完全迁移到新API

```go
// 使用新的工厂函数
manager, err := config.CreateIntegratedManager(
    defaultConfig,
    "config.yaml",
    "./configs",
)
```

## 迁移示例

### 基本使用

**旧代码:**
```go
manager := config.NewManager(defaultConfig)
err := manager.Load()
if err != nil {
    log.Fatal(err)
}

cfg, err := manager.GetConfig()
if err != nil {
    log.Fatal(err)
}

err = manager.UpdateField(func(c *Config) {
    c.Server.Port = 8080
})
```

**新代码（兼容性层）:**
```go
manager := config.Default(defaultConfig)
err := manager.Load()
if err != nil {
    log.Fatal(err)
}

cfg, err := manager.GetConfig()
if err != nil {
    log.Fatal(err)
}

err = manager.UpdateField(func(c *Config) {
    c.Server.Port = 8080
})
```

**新代码（完全迁移）:**
```go
manager, err := config.CreateIntegratedManager(
    defaultConfig,
    "config.yaml",
    "./configs",
)
if err != nil {
    log.Fatal(err)
}

cfg, err := manager.GetConfigSafe()
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
err = manager.UpdateConfigSafe(ctx, func(c *Config) {
    c.Server.Port = 8080
})
```

### 钩子系统

**旧代码:**
```go
manager.SetHook(config.Info, func(ctx config.HookContext) {
    log.Printf("Info: %s", ctx.Message)
})
```

**新代码:**
```go
type InfoHook struct{}

func (h *InfoHook) Handle(ctx *config.HookContext) error {
    log.Printf("Info: %s", ctx.Message)
    return nil
}

func (h *InfoHook) Priority() int { return 0 }
func (h *InfoHook) IsAsync() bool { return false }

// 使用兼容性层
manager := config.Default(defaultConfig)
manager.EnableAdvancedFeatures()
advancedManager := manager.GetAdvancedManager()
err := advancedManager.RegisterHook(config.HookTypeInfo, &InfoHook{})
```

### 文件监听

**旧代码:**
```go
// 旧版本没有停止监听功能
manager.Load() // 自动开始监听
```

**新代码:**
```go
manager := config.Default(defaultConfig)
manager.EnableAdvancedFeatures()

// 启动监听
err := manager.StartWatching()
if err != nil {
    log.Fatal(err)
}

// 停止监听
defer manager.StopWatching()
```

### 高级功能

**新功能示例:**
```go
manager := config.Default(defaultConfig)
manager.EnableAdvancedFeatures()
advancedManager := manager.GetAdvancedManager()

// 获取调度池
pool := advancedManager.GetSchedulerPool()
fmt.Printf("调度池状态: %t\n", pool.IsRunning())

// 提交任务
err := advancedManager.SubmitTask("my-task", config.TaskTypeHook, 0, func(ctx context.Context) error {
    fmt.Println("执行自定义任务")
    return nil
})

// 获取配置存储
store := advancedManager.GetConfigStore()
stats := store.GetStats()
fmt.Printf("配置版本: %d\n", stats.CurrentVersion)

// 获取拦截统计
stats := advancedManager.GetInterceptionStats()
fmt.Printf("拦截统计: %+v\n", stats)
```

## 迁移策略

### 策略 1: 渐进式迁移（推荐）

1. 使用兼容性层替换现有代码
2. 逐步启用高级功能
3. 测试每个功能
4. 最终迁移到新API

### 策略 2: 一次性迁移

1. 直接使用新的API
2. 重写所有相关代码
3. 全面测试

### 策略 3: 混合模式

1. 新功能使用新API
2. 现有功能保持兼容性层
3. 逐步替换

## 测试迁移

### 兼容性检查

```go
// 检查兼容性
compatibility := config.CheckCompatibility()
fmt.Printf("兼容性: %+v\n", compatibility)

// 验证兼容性
compatible, issues := config.ValidateCompatibility("1.5.0")
if !compatible {
    for _, issue := range issues {
        log.Printf("兼容性问题: %s", issue)
    }
}
```

### 功能测试

```go
// 测试基本功能
manager := config.Default(defaultConfig)
cfg, err := manager.GetConfig()
if err != nil {
    t.Errorf("获取配置失败: %v", err)
}

// 测试高级功能
manager.EnableAdvancedFeatures()
if !manager.IsUsingAdvancedFeatures() {
    t.Error("高级功能未启用")
}
```

## 性能考虑

### 内存使用

- 新版本使用更多内存来提供线程安全
- 配置快照功能会占用额外内存
- 调度池会预分配goroutine

### CPU使用

- 新版本在高并发场景下性能更好
- 防抖机制减少不必要的处理
- 调度池避免频繁创建goroutine

### 网络/IO

- 文件监听更加高效
- 支持优雅关闭，避免资源泄露
- 更好的错误处理和恢复

## 故障排除

### 常见问题

1. **编译错误**: 检查导入路径和API变更
2. **运行时错误**: 检查配置文件路径和权限
3. **性能问题**: 调整调度池大小和缓冲区
4. **内存泄露**: 确保正确关闭管理器

### 调试技巧

```go
// 启用调试信息
manager := config.Default(defaultConfig)
manager.EnableAdvancedFeatures()

// 获取状态信息
status := manager.GetStatus()
fmt.Printf("状态: %+v\n", status)

// 获取拦截统计
stats := manager.GetAdvancedManager().GetInterceptionStats()
fmt.Printf("拦截统计: %+v\n", stats)
```

## 最佳实践

1. **使用兼容性层**: 开始时使用兼容性层，逐步迁移
2. **测试驱动**: 为每个迁移步骤编写测试
3. **监控性能**: 监控迁移前后的性能变化
4. **错误处理**: 改进错误处理逻辑
5. **文档更新**: 更新相关文档和注释

## 获取帮助

- 查看API文档
- 运行示例代码
- 检查兼容性信息
- 提交Issue或PR

## 版本历史

- **2.0.0**: 重构架构，新增高级功能
- **1.x**: 传统架构，基本功能

---

*本指南会随着版本更新而持续更新。*