package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Configurable 配置接口，所有配置结构体都应该实现此接口
type Configurable interface {
	// 可以添加必要的方法约束，比如验证方法
	// Validate() error
}

// ConfigManager 配置管理器接口（保持向后兼容）
type ConfigManager[T Configurable] interface {
	Init(ctx context.Context, handles ...HandlerFunc) error
	GetConfig() (*T, error)
	UpdateField(ctx context.Context, updateFunc func(*T)) error
	SetHook(pattern HookPattern, handler HookHandlerFunc) *Manager[T]
}

// Manager 传统配置管理器（保持向后兼容）
type Manager[T Configurable] struct {
	config             *T            // 全局配置对象
	vp                 *viper.Viper  // Viper 实例
	rwMutex            sync.RWMutex  // 读写锁
	lastChange         time.Time     // 上次触发时间（用于防抖）
	debounceDur        time.Duration // 防抖间隔
	hooks              *Hook         // hook
	pathName           string        // 配置文件
	opts               *Option       // 设置选项
	optsInit           bool          // 初始化选项
	initializeValidate bool          // 初始化验证
	defaultConfig      *T            // default config

	// 集成重构后的组件
	refactoredManager *RefactoredManager[T] // 重构后的管理器
	migrationMode     bool                  // 迁移模式标志
}

// NewManager 创建传统配置管理器（保持向后兼容）
func NewManager[T Configurable](defaultConfig *T) *Manager[T] {
	return &Manager[T]{
		config:        defaultConfig,
		vp:            viper.New(),
		lastChange:    time.Time{},
		hooks:         NewHook(),
		defaultConfig: defaultConfig,
		migrationMode: false, // 默认使用传统模式
	}
}

// EnableRefactoredMode 启用重构模式
// 这个方法允许用户逐步迁移到新的重构架构
func (m *Manager[T]) EnableRefactoredMode() *Manager[T] {
	m.migrationMode = true
	if m.refactoredManager == nil {
		m.refactoredManager = NewRefactoredManager(m.defaultConfig)
	}
	return m
}

// GetRefactoredManager 获取重构后的管理器
// 用于访问新功能，如调度池、线程安全存储等
func (m *Manager[T]) GetRefactoredManager() *RefactoredManager[T] {
	if m.refactoredManager == nil {
		m.refactoredManager = NewRefactoredManager(m.defaultConfig)
	}
	return m.refactoredManager
}

// InitWithOptions 使用选项初始化管理器
// 这是新的初始化方法，支持重构后的选项系统
func (m *Manager[T]) InitWithOptions(ctx context.Context, option *ImmutableOption) error {
	if m.migrationMode {
		// 使用重构后的管理器
		if m.refactoredManager == nil {
			m.refactoredManager = NewRefactoredManager(m.defaultConfig)
		}
		return m.refactoredManager.Initialize(ctx, option)
	}

	// 传统初始化逻辑（保持向后兼容）
	return m.initTraditional(ctx, option)
}

// initTraditional 传统初始化方法
func (m *Manager[T]) initTraditional(ctx context.Context, option *ImmutableOption) error {
	m.rwMutex.Lock()
	defer m.rwMutex.Unlock()

	if option != nil {
		m.pathName = option.GetFullPath()
		m.debounceDur = option.GetDebounceDuration()
	} else {
		m.pathName = OptionFilepath + "/" + OptionFilename
		m.debounceDur = OptionDebounceDur
	}

	// 设置Viper配置
	m.vp.SetConfigFile(m.pathName)
	m.vp.SetConfigType("yaml")

	// 读取配置文件
	if err := m.vp.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// 解析配置
	if err := m.vp.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	m.optsInit = true
	return nil
}

// LoadWithOptions 使用选项加载配置
func (m *Manager[T]) LoadWithOptions(ctx context.Context, option *ImmutableOption, handlers ...ChangeHandler[T]) error {
	if m.migrationMode && m.refactoredManager != nil {
		// 使用重构后的管理器
		if !m.refactoredManager.IsInitialized() {
			if err := m.refactoredManager.Initialize(ctx, option); err != nil {
				return err
			}
		}
		return m.refactoredManager.Load(ctx, handlers...)
	}

	// 传统加载逻辑
	return m.initTraditional(ctx, option)
}

// StartWatchingWithOptions 使用选项启动监听
func (m *Manager[T]) StartWatchingWithOptions(ctx context.Context) error {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.StartWatching(ctx)
	}

	// 传统监听逻辑（简化实现）
	return fmt.Errorf("traditional watching not implemented, please use EnableRefactoredMode()")
}

// StopWatching 停止监听
func (m *Manager[T]) StopWatching(timeout time.Duration) error {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.StopWatching(timeout)
	}

	// 传统停止逻辑
	return nil
}

// RegisterHookWithType 注册类型化钩子
func (m *Manager[T]) RegisterHookWithType(hookType HookType, handler HookHandler) error {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.RegisterHook(hookType, handler)
	}

	// 传统钩子注册（转换为新格式）
	if m.hooks != nil && int(hookType) < len(m.hooks.Handles) {
		// 将新的HookHandler转换为传统的HookHandlerFunc
		m.hooks.Handles[hookType] = func(ctx HookContext) {
			newCtx := &HookContext{
				Type:      hookType,
				Message:   ctx.Message,
				Config:    ctx.Config,
				Error:     ctx.Error,
				Metadata:  make(map[string]interface{}),
				Timestamp: time.Now(),
			}
			handler.Handle(newCtx)
		}
	}

	return nil
}

// GetStatus 获取管理器状态
func (m *Manager[T]) GetStatus() *ManagerStatus {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.GetStatus()
	}

	// 传统状态
	return &ManagerStatus{
		Initialized:   m.optsInit,
		Watching:      false,
		LastUpdate:    m.lastChange,
		ConfigVersion: 0,
		ErrorCount:    0,
		LastError:     nil,
		Uptime:        0,
		StartTime:     time.Now(),
	}
}

// Close 关闭管理器
func (m *Manager[T]) Close(timeout time.Duration) error {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.Close(timeout)
	}

	// 传统关闭逻辑
	m.rwMutex.Lock()
	defer m.rwMutex.Unlock()
	m.optsInit = false
	return nil
}

// Init 初始化管理器（保持向后兼容）
func (m *Manager[T]) Init(ctx context.Context, handles ...HandlerFunc) error {
	if m.migrationMode && m.refactoredManager != nil {
		// 在重构模式下，使用默认选项初始化
		option, err := NewOptionBuilder().Build()
		if err != nil {
			return fmt.Errorf("failed to build default option: %w", err)
		}
		return m.refactoredManager.Initialize(ctx, option)
	}

	// 传统初始化逻辑
	return m.initTraditionalWithHandlers(ctx, handles...)
}

// initTraditionalWithHandlers 传统初始化方法（带处理器）
func (m *Manager[T]) initTraditionalWithHandlers(ctx context.Context, handles ...HandlerFunc) error {
	m.rwMutex.Lock()
	defer m.rwMutex.Unlock()

	// 使用默认选项
	if m.opts == nil {
		m.opts = NewOption()
	}

	m.pathName = m.opts.File()
	m.debounceDur = m.opts.DebounceDur.ToValue()

	// 设置Viper配置
	m.vp.SetConfigFile(m.pathName)
	m.vp.SetConfigType("yaml")

	// 读取配置文件
	if err := m.vp.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// 解析配置
	if err := m.vp.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	m.optsInit = true
	return nil
}

// GetConfig 获取配置（保持向后兼容）
func (m *Manager[T]) GetConfig() (*T, error) {
	return m.GetConfigSafe()
}

// UpdateField 更新字段（保持向后兼容）
func (m *Manager[T]) UpdateField(ctx context.Context, updateFunc func(*T)) error {
	return m.UpdateConfigSafe(ctx, updateFunc)
}

// SetHook 设置钩子（保持向后兼容）
func (m *Manager[T]) SetHook(pattern HookPattern, handler HookHandlerFunc) *Manager[T] {
	if m.hooks != nil && int(pattern) < len(m.hooks.Handles) {
		m.hooks.Handles[pattern] = handler
	}
	return m
}
func (m *Manager[T]) GetConfigSafe() (*T, error) {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.GetConfig()
	}

	// 传统获取配置
	m.rwMutex.RLock()
	defer m.rwMutex.RUnlock()

	if m.config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// 返回配置副本
	configCopy := *m.config
	return &configCopy, nil
}

// UpdateConfigSafe 线程安全地更新配置
func (m *Manager[T]) UpdateConfigSafe(ctx context.Context, updateFunc func(*T)) error {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.UpdateConfig(ctx, updateFunc)
	}

	// 传统更新配置
	m.rwMutex.Lock()
	defer m.rwMutex.Unlock()

	if m.config == nil {
		return fmt.Errorf("config is nil")
	}

	updateFunc(m.config)
	m.lastChange = time.Now()
	return nil
}

// GetSchedulerPool 获取调度池（仅重构模式可用）
func (m *Manager[T]) GetSchedulerPool() *SchedulerPool {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.GetSchedulerPool()
	}
	return nil
}

// GetConfigStore 获取配置存储（仅重构模式可用）
func (m *Manager[T]) GetConfigStore() *ConfigStore[T] {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.GetConfigStore()
	}
	return nil
}

// GetFileWatcher 获取文件监听器（仅重构模式可用）
func (m *Manager[T]) GetFileWatcher() *FileWatcher {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.GetFileWatcher()
	}
	return nil
}

// GetHookRegistry 获取钩子注册表（仅重构模式可用）
func (m *Manager[T]) GetHookRegistry() *HookRegistry {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.GetHookRegistry()
	}
	return nil
}

// IsRefactoredMode 检查是否为重构模式
func (m *Manager[T]) IsRefactoredMode() bool {
	return m.migrationMode
}

// MigrateToRefactored 迁移到重构模式
// 这个方法帮助用户从传统模式平滑迁移到重构模式
func (m *Manager[T]) MigrateToRefactored(ctx context.Context, option *ImmutableOption) error {
	if m.migrationMode {
		return fmt.Errorf("already in refactored mode")
	}

	// 创建重构后的管理器
	m.refactoredManager = NewRefactoredManager(m.defaultConfig)

	// 初始化重构后的管理器
	if err := m.refactoredManager.Initialize(ctx, option); err != nil {
		return fmt.Errorf("failed to initialize refactored manager: %w", err)
	}

	// 如果传统管理器已经有配置，迁移配置
	if m.config != nil {
		if err := m.refactoredManager.GetConfigStore().SetConfig(m.config); err != nil {
			return fmt.Errorf("failed to migrate config: %w", err)
		}
	}

	// 启用重构模式
	m.migrationMode = true

	return nil
}

// GetInterceptionStats 获取拦截统计信息（仅重构模式可用）
func (m *Manager[T]) GetInterceptionStats() map[string]interface{} {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.GetInterceptionStats()
	}
	return map[string]interface{}{
		"refactored_mode": false,
		"message":         "interception stats only available in refactored mode",
	}
}

// SubmitTask 提交任务到调度池（仅重构模式可用）
func (m *Manager[T]) SubmitTask(taskID string, taskType TaskType, priority int, fn func(context.Context) error) error {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.SubmitTask(taskID, taskType, priority, fn)
	}
	return fmt.Errorf("task submission only available in refactored mode")
}

// SubmitGoroutine 提交goroutine到管理器（仅重构模式可用）
func (m *Manager[T]) SubmitGoroutine(fn func()) error {
	if m.migrationMode && m.refactoredManager != nil {
		return m.refactoredManager.SubmitGoroutine(fn)
	}
	return fmt.Errorf("goroutine submission only available in refactored mode")
}
