package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// RefactoredConfigManager 重构后的配置管理器接口
//
// RefactoredConfigManager 定义了重构后配置管理器的完整接口，
// 支持泛型以确保类型安全，并提供了丰富的配置管理功能。
//
// 接口特性：
// - 泛型支持：T必须实现Configurable接口
// - 生命周期管理：从初始化到关闭的完整生命周期
// - 异步处理：支持异步配置加载和更新
// - 钩子系统：在关键节点支持自定义逻辑注入
// - 文件监听：自动监听配置文件变更
// - 状态查询：提供详细的运行状态信息
//
// 使用流程：
// 1. 创建管理器实例
// 2. 调用Initialize进行初始化
// 3. 调用Load加载配置
// 4. 可选调用StartWatching启动文件监听
// 5. 使用GetConfig获取配置或UpdateConfig更新配置
// 6. 最后调用Close清理资源
type RefactoredConfigManager[T Configurable] interface {
	// Initialize 初始化配置管理器
	Initialize(ctx context.Context, option *ImmutableOption) error

	// Load 加载配置
	Load(ctx context.Context, handlers ...ChangeHandler[T]) error

	// GetConfig 获取配置
	GetConfig() (*T, error)

	// UpdateConfig 更新配置
	UpdateConfig(ctx context.Context, updateFunc func(*T)) error

	// RegisterHook 注册钩子
	RegisterHook(hookType HookType, handler HookHandler) error

	// StartWatching 启动监听
	StartWatching(ctx context.Context) error

	// StopWatching 停止监听
	StopWatching(timeout time.Duration) error

	// Close 关闭管理器
	Close(timeout time.Duration) error

	// GetStatus 获取状态
	GetStatus() *ManagerStatus
}

// ChangeHandler 配置变更处理器
type ChangeHandler[T Configurable] func(ctx context.Context, oldConfig, newConfig *T) error

// ManagerStatus 管理器状态
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

// RefactoredManager 重构后的配置管理器实现
//
// RefactoredManager 是RefactoredConfigManager接口的完整实现，
// 集成了所有重构后的组件，提供了企业级的配置管理功能。
//
// 架构组件：
// - HookRegistry: 钩子注册和管理
// - HookExecutor: 钩子执行引擎
// - SchedulerPool: goroutine调度池
// - FileWatcher: 文件变更监听
// - ConfigStore: 线程安全配置存储
// - ViperWrapper: Viper包装器，拦截goroutine创建
// - GoroutineManager: 统一的goroutine管理
//
// 核心特性：
// - 完全线程安全的配置访问
// - 统一的goroutine调度，避免资源泄漏
// - 丰富的钩子系统，支持自定义逻辑
// - 自动文件监听和热重载
// - 配置版本管理和快照
// - 优雅关闭和资源清理
//
// 状态管理：
// 管理器维护详细的运行状态，包括初始化状态、监听状态、
// 错误计数、版本信息等，便于监控和调试。
type RefactoredManager[T Configurable] struct {
	// 配置数据
	config        *T           // 当前配置对象
	defaultConfig *T           // 默认配置对象
	viper         *viper.Viper // Viper实例

	// 核心组件
	hookRegistry     *HookRegistry          // 钩子注册表
	hookExecutor     *HookExecutor          // 钩子执行器
	schedulerPool    *SchedulerPool         // 调度池
	fileWatcher      *FileWatcher           // 文件监听器
	configStore      *ConfigStore[T]        // 配置存储
	viperWrapper     *ScheduledViperWrapper // Viper包装器
	goroutineManager *GoroutineManager      // Goroutine管理器

	// 状态管理
	ctx         context.Context    // 上下文
	cancel      context.CancelFunc // 取消函数
	status      *ManagerStatus     // 状态信息
	statusMutex sync.RWMutex       // 状态锁

	// 配置选项
	option      *ImmutableOption // 配置选项
	initialized int32            // 初始化状态（原子操作）
	watching    int32            // 监听状态（原子操作）

	// 变更处理器
	changeHandlers []ChangeHandler[T] // 配置变更处理器列表
	handlersMutex  sync.RWMutex       // 处理器锁

	// 同步控制
	initMutex sync.Mutex   // 初始化锁
	mutex     sync.RWMutex // 主锁
}

// NewRefactoredManager 创建新的重构配置管理器
//
// 创建一个新的RefactoredManager实例，这是使用重构后配置管理器
// 的入口点。
//
// 参数：
//
//	defaultConfig: 默认配置对象，不能为nil，用作配置模板和回退值
//
// 返回值：
//
//	*RefactoredManager[T]: 新创建的配置管理器实例
//
// 初始化状态：
// - 设置默认配置和当前配置
// - 创建新的Viper实例
// - 初始化状态管理结构
// - 准备变更处理器列表
//
// 注意：
// - 创建后需要调用Initialize方法进行完整初始化
// - defaultConfig会被用作配置模板，应该包含所有必要的字段
// - 如果defaultConfig为nil，会触发panic
//
// 使用示例：
//
//	manager := NewRefactoredManager(&MyConfig{
//	    Database: DatabaseConfig{Host: "localhost"},
//	    Server:   ServerConfig{Port: 8080},
//	})
func NewRefactoredManager[T Configurable](defaultConfig *T) *RefactoredManager[T] {
	if defaultConfig == nil {
		panic("default config cannot be nil")
	}

	manager := &RefactoredManager[T]{
		config:        defaultConfig,
		defaultConfig: defaultConfig,
		viper:         viper.New(),
		status: &ManagerStatus{
			StartTime: time.Now(),
		},
		changeHandlers: make([]ChangeHandler[T], 0),
	}

	return manager
}

// Initialize 初始化配置管理器
func (m *RefactoredManager[T]) Initialize(ctx context.Context, option *ImmutableOption) error {
	// 检查是否已经初始化
	if atomic.LoadInt32(&m.initialized) == 1 {
		return fmt.Errorf("manager already initialized")
	}

	m.initMutex.Lock()
	defer m.initMutex.Unlock()

	// 双重检查
	if atomic.LoadInt32(&m.initialized) == 1 {
		return fmt.Errorf("manager already initialized")
	}

	if option == nil {
		return fmt.Errorf("option cannot be nil")
	}

	// 设置上下文
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.option = option

	// 初始化组件
	if err := m.initializeComponents(); err != nil {
		m.cleanup()
		return fmt.Errorf("failed to initialize components: %w", err)
	}

	// 设置Viper配置
	if err := m.setupViper(); err != nil {
		m.cleanup()
		return fmt.Errorf("failed to setup viper: %w", err)
	}

	// 执行初始化钩子
	if err := m.executeHook(HookTypeInit, "manager initialized", nil, nil); err != nil {
		// 初始化钩子失败不应该阻止初始化过程，只记录错误
		m.updateStatus(func(s *ManagerStatus) {
			s.LastError = err
			s.ErrorCount++
		})
	}

	// 标记为已初始化
	atomic.StoreInt32(&m.initialized, 1)
	m.updateStatus(func(s *ManagerStatus) {
		s.Initialized = true
	})

	return nil
}

// initializeComponents 初始化所有组件
func (m *RefactoredManager[T]) initializeComponents() error {
	// 初始化钩子注册表
	m.hookRegistry = NewHookRegistry()

	// 初始化调度池
	m.schedulerPool = NewSchedulerPool(
		m.option.GetMaxWorkers(),
		m.option.GetBufferSize(),
	)

	// 启动调度池
	if err := m.schedulerPool.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start scheduler pool: %w", err)
	}

	// 初始化goroutine管理器
	m.goroutineManager = NewGoroutineManager(m.schedulerPool)

	// 初始化Viper包装器（拦截Viper的goroutine创建）
	m.viperWrapper = NewScheduledViperWrapper(m.viper, m.schedulerPool)
	if err := m.viperWrapper.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize viper wrapper: %w", err)
	}

	// 初始化钩子执行器
	m.hookExecutor = NewHookExecutor(
		m.hookRegistry,
		m.schedulerPool,
		30*time.Second, // 默认30秒超时
	)

	// 初始化配置存储
	m.configStore = NewConfigStore[T](m.defaultConfig, 10) // 最多保留10个快照

	// 初始化防抖器和文件监听器
	debouncer := NewDebouncer(m.option.GetDebounceDuration())
	m.fileWatcher = NewFileWatcher(
		m.option.GetFullPath(),
		debouncer,
		m.schedulerPool,
	)

	return nil
}

// setupViper 设置Viper配置
func (m *RefactoredManager[T]) setupViper() error {
	// 设置配置文件
	m.viperWrapper.SetConfigFile(m.option.GetFullPath())

	// 设置配置文件类型
	m.viperWrapper.SetConfigType("yaml") // 默认使用yaml

	// 尝试读取配置文件
	if err := m.viperWrapper.ReadInConfig(); err != nil {
		// 如果文件不存在，创建默认配置文件
		if err := m.ensureConfigFile(); err != nil {
			return fmt.Errorf("failed to ensure config file: %w", err)
		}

		// 重新尝试读取
		if err := m.viperWrapper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config after creation: %w", err)
		}
	}

	return nil
}

// ensureConfigFile 确保配置文件存在
func (m *RefactoredManager[T]) ensureConfigFile() error {
	return m.ensureConfigFileWithOption(m.option)
}

// Load 加载配置
func (m *RefactoredManager[T]) Load(ctx context.Context, handlers ...ChangeHandler[T]) error {
	if atomic.LoadInt32(&m.initialized) == 0 {
		return fmt.Errorf("manager not initialized")
	}

	// 添加变更处理器
	if len(handlers) > 0 {
		m.handlersMutex.Lock()
		m.changeHandlers = append(m.changeHandlers, handlers...)
		m.handlersMutex.Unlock()
	}

	// 执行加载前钩子
	if err := m.executeHook(HookTypeBeforeLoad, "loading configuration", nil, nil); err != nil {
		m.updateErrorStatus(err)
	}

	// 加载配置
	if err := m.loadConfiguration(); err != nil {
		m.updateErrorStatus(err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 执行加载后钩子
	if err := m.executeHook(HookTypeAfterLoad, "configuration loaded", m.config, nil); err != nil {
		m.updateErrorStatus(err)
	}

	return nil
}

// loadConfiguration 加载配置数据
func (m *RefactoredManager[T]) loadConfiguration() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 解析配置到结构体
	var newConfig T
	if err := m.viperWrapper.Unmarshal(&newConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 保存旧配置用于比较
	oldConfig := m.config

	// 更新配置存储
	if err := m.configStore.SetConfig(&newConfig); err != nil {
		return fmt.Errorf("failed to update config store: %w", err)
	}

	// 更新当前配置
	m.config = &newConfig

	// 更新状态
	m.updateStatus(func(s *ManagerStatus) {
		s.LastUpdate = time.Now()
		s.ConfigVersion = m.configStore.GetVersion()
	})

	// 执行变更处理器
	if oldConfig != nil {
		go m.executeChangeHandlers(oldConfig, &newConfig)
	}

	return nil
}

// executeChangeHandlers 执行配置变更处理器
func (m *RefactoredManager[T]) executeChangeHandlers(oldConfig, newConfig *T) {
	m.handlersMutex.RLock()
	handlers := make([]ChangeHandler[T], len(m.changeHandlers))
	copy(handlers, m.changeHandlers)
	m.handlersMutex.RUnlock()

	for _, handler := range handlers {
		if handler != nil {
			// 在调度池中异步执行处理器
			taskID := fmt.Sprintf("change-handler-%d", time.Now().UnixNano())
			task := NewConfigTask(taskID, TaskTypeUpdate, 0, func(ctx context.Context) error {
				return handler(ctx, oldConfig, newConfig)
			})

			if err := m.schedulerPool.Submit(task); err != nil {
				// 如果调度池提交失败，直接在当前goroutine执行
				if err := handler(m.ctx, oldConfig, newConfig); err != nil {
					m.updateErrorStatus(fmt.Errorf("change handler failed: %w", err))
				}
			}
		}
	}
}

// GetConfig 获取配置
func (m *RefactoredManager[T]) GetConfig() (*T, error) {
	if atomic.LoadInt32(&m.initialized) == 0 {
		return nil, fmt.Errorf("manager not initialized")
	}

	return m.configStore.GetConfig()
}

// UpdateConfig 更新配置
func (m *RefactoredManager[T]) UpdateConfig(ctx context.Context, updateFunc func(*T)) error {
	if atomic.LoadInt32(&m.initialized) == 0 {
		return fmt.Errorf("manager not initialized")
	}

	if updateFunc == nil {
		return fmt.Errorf("update function cannot be nil")
	}

	// 执行更新前钩子
	if err := m.executeHook(HookTypeBeforeUpdate, "updating configuration", m.config, nil); err != nil {
		m.updateErrorStatus(err)
	}

	// 使用配置存储的原子更新功能
	err := m.configStore.UpdateConfig(func(config *T) (*T, error) {
		// 创建配置副本
		configCopy := *config

		// 执行更新函数
		updateFunc(&configCopy)

		return &configCopy, nil
	})

	if err != nil {
		m.updateErrorStatus(err)
		return fmt.Errorf("failed to update config: %w", err)
	}

	// 更新当前配置引用
	m.mutex.Lock()
	newConfig, _ := m.configStore.GetConfig()
	oldConfig := m.config
	m.config = newConfig
	m.mutex.Unlock()

	// 更新状态
	m.updateStatus(func(s *ManagerStatus) {
		s.LastUpdate = time.Now()
		s.ConfigVersion = m.configStore.GetVersion()
	})

	// 执行更新后钩子
	if err := m.executeHook(HookTypeAfterUpdate, "configuration updated", newConfig, nil); err != nil {
		m.updateErrorStatus(err)
	}

	// 执行变更处理器
	if oldConfig != nil {
		go m.executeChangeHandlers(oldConfig, newConfig)
	}

	return nil
}

// RegisterHook 注册钩子
func (m *RefactoredManager[T]) RegisterHook(hookType HookType, handler HookHandler) error {
	if m.hookRegistry == nil {
		return fmt.Errorf("hook registry not initialized")
	}

	return m.hookRegistry.RegisterHook(hookType, handler)
}

// StartWatching 启动监听
func (m *RefactoredManager[T]) StartWatching(ctx context.Context) error {
	if atomic.LoadInt32(&m.initialized) == 0 {
		return fmt.Errorf("manager not initialized")
	}

	// 检查是否已经在监听
	if atomic.LoadInt32(&m.watching) == 1 {
		return fmt.Errorf("already watching")
	}

	// 添加文件变更处理器
	m.fileWatcher.AddHandlerWithID(NewFileChangeHandler(
		"config-reload",
		m.handleFileChange,
	).SetPriority(0).SetAsync(true))

	// 启动文件监听器
	if err := m.fileWatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// 标记为正在监听
	atomic.StoreInt32(&m.watching, 1)
	m.updateStatus(func(s *ManagerStatus) {
		s.Watching = true
	})

	return nil
}

// handleFileChange 处理文件变更事件
func (m *RefactoredManager[T]) handleFileChange(event fsnotify.Event) error {
	// 重新读取配置文件
	if err := m.viperWrapper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to reload config file: %w", err)
	}

	// 重新加载配置
	if err := m.loadConfiguration(); err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	return nil
}

// StopWatching 停止监听
func (m *RefactoredManager[T]) StopWatching(timeout time.Duration) error {
	if atomic.LoadInt32(&m.watching) == 0 {
		return nil // 已经停止
	}

	// 停止文件监听器
	if m.fileWatcher != nil {
		if err := m.fileWatcher.Stop(timeout); err != nil {
			return fmt.Errorf("failed to stop file watcher: %w", err)
		}
	}

	// 标记为已停止监听
	atomic.StoreInt32(&m.watching, 0)
	m.updateStatus(func(s *ManagerStatus) {
		s.Watching = false
	})

	return nil
}

// Close 关闭管理器
func (m *RefactoredManager[T]) Close(timeout time.Duration) error {
	// 停止监听
	if err := m.StopWatching(timeout); err != nil {
		// 记录错误但继续关闭过程
		m.updateErrorStatus(err)
	}

	// 执行清理
	m.cleanup()

	// 重置状态
	atomic.StoreInt32(&m.initialized, 0)
	atomic.StoreInt32(&m.watching, 0)

	m.updateStatus(func(s *ManagerStatus) {
		s.Initialized = false
		s.Watching = false
	})

	return nil
}

// cleanup 清理资源
func (m *RefactoredManager[T]) cleanup() {
	// 取消上下文
	if m.cancel != nil {
		m.cancel()
	}

	// 停止调度池
	if m.schedulerPool != nil {
		m.schedulerPool.Stop(5 * time.Second)
	}

	// 清理钩子注册表
	if m.hookRegistry != nil {
		m.hookRegistry.ClearAll()
	}
}

// GetStatus 获取状态
func (m *RefactoredManager[T]) GetStatus() *ManagerStatus {
	m.statusMutex.RLock()
	defer m.statusMutex.RUnlock()

	// 创建状态副本
	status := *m.status
	status.Uptime = time.Since(status.StartTime)

	return &status
}

// updateStatus 更新状态
func (m *RefactoredManager[T]) updateStatus(updateFunc func(*ManagerStatus)) {
	m.statusMutex.Lock()
	defer m.statusMutex.Unlock()

	updateFunc(m.status)
}

// updateErrorStatus 更新错误状态
func (m *RefactoredManager[T]) updateErrorStatus(err error) {
	m.updateStatus(func(s *ManagerStatus) {
		s.LastError = err
		s.ErrorCount++
	})
}

// executeHook 执行钩子
func (m *RefactoredManager[T]) executeHook(hookType HookType, message string, config interface{}, err error) error {
	if m.hookExecutor == nil {
		return nil // 钩子执行器未初始化，跳过
	}

	hookCtx := &HookContext{
		Type:      hookType,
		Message:   message,
		Config:    config,
		Error:     err,
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	return m.hookExecutor.Execute(m.ctx, hookCtx)
}

// IsInitialized 检查是否已初始化
func (m *RefactoredManager[T]) IsInitialized() bool {
	return atomic.LoadInt32(&m.initialized) == 1
}

// IsWatching 检查是否正在监听
func (m *RefactoredManager[T]) IsWatching() bool {
	return atomic.LoadInt32(&m.watching) == 1
}

// GetConfigStore 获取配置存储（用于高级操作）
func (m *RefactoredManager[T]) GetConfigStore() *ConfigStore[T] {
	return m.configStore
}

// GetSchedulerPool 获取调度池（用于高级操作）
func (m *RefactoredManager[T]) GetSchedulerPool() *SchedulerPool {
	return m.schedulerPool
}

// GetHookRegistry 获取钩子注册表（用于高级操作）
func (m *RefactoredManager[T]) GetHookRegistry() *HookRegistry {
	return m.hookRegistry
}

// GetFileWatcher 获取文件监听器（用于高级操作）
func (m *RefactoredManager[T]) GetFileWatcher() *FileWatcher {
	return m.fileWatcher
}

// ensureConfigFileWithOption 确保配置文件存在（使用指定选项）
func (m *RefactoredManager[T]) ensureConfigFileWithOption(option *ImmutableOption) error {
	configPath := option.GetFullPath()

	// 检查文件是否存在
	if _, err := os.Stat(configPath); err == nil {
		return nil // 文件已存在
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 序列化默认配置
	data, err := yaml.Marshal(m.defaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	// 写入配置文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// 执行信息钩子
	if err := m.executeHook(HookTypeInfo, fmt.Sprintf("default config file created: %s", configPath), nil, nil); err != nil {
		// 钩子执行失败不应该影响文件创建过程
		m.updateErrorStatus(err)
	}

	return nil
}

// GetViperWrapper 获取Viper包装器（用于高级操作）
func (m *RefactoredManager[T]) GetViperWrapper() *ScheduledViperWrapper {
	return m.viperWrapper
}

// GetGoroutineManager 获取Goroutine管理器（用于高级操作）
func (m *RefactoredManager[T]) GetGoroutineManager() *GoroutineManager {
	return m.goroutineManager
}

// StartWatchingWithViper 使用Viper包装器启动监听
// 这个方法展示了如何使用拦截的Viper功能
func (m *RefactoredManager[T]) StartWatchingWithViper(ctx context.Context) error {
	if atomic.LoadInt32(&m.initialized) == 0 {
		return fmt.Errorf("manager not initialized")
	}

	// 检查是否已经在监听
	if atomic.LoadInt32(&m.watching) == 1 {
		return fmt.Errorf("already watching")
	}

	// 注册配置变更回调到Viper包装器
	if err := m.viperWrapper.OnConfigChange("manager-reload", func(event fsnotify.Event) {
		if err := m.handleFileChange(event); err != nil {
			m.updateErrorStatus(fmt.Errorf("failed to handle config change: %w", err))
		}
	}); err != nil {
		return fmt.Errorf("failed to register config change callback: %w", err)
	}

	// 启动Viper监听（使用调度池）
	if err := m.viperWrapper.WatchConfig(ctx); err != nil {
		return fmt.Errorf("failed to start viper watching: %w", err)
	}

	// 标记为正在监听
	atomic.StoreInt32(&m.watching, 1)
	m.updateStatus(func(s *ManagerStatus) {
		s.Watching = true
	})

	return nil
}

// SubmitTask 提交任务到调度池
// 提供统一的任务提交接口
func (m *RefactoredManager[T]) SubmitTask(taskID string, taskType TaskType, priority int, fn func(context.Context) error) error {
	if m.schedulerPool == nil || !m.schedulerPool.IsRunning() {
		return fmt.Errorf("scheduler pool not available")
	}

	task := NewConfigTask(taskID, taskType, priority, fn)
	return m.schedulerPool.Submit(task)
}

// SubmitGoroutine 提交goroutine到管理器
// 确保所有goroutine都通过统一的调度机制
func (m *RefactoredManager[T]) SubmitGoroutine(fn func()) error {
	if m.goroutineManager == nil {
		return fmt.Errorf("goroutine manager not available")
	}

	return m.goroutineManager.Go(fn)
}

// SubmitGoroutineWithContext 提交带上下文的goroutine到管理器
func (m *RefactoredManager[T]) SubmitGoroutineWithContext(ctx context.Context, fn func(context.Context)) error {
	if m.goroutineManager == nil {
		return fmt.Errorf("goroutine manager not available")
	}

	return m.goroutineManager.GoWithContext(ctx, fn)
}

// GetInterceptionStats 获取拦截统计信息
func (m *RefactoredManager[T]) GetInterceptionStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Viper拦截统计
	if m.viperWrapper != nil {
		interceptor := m.viperWrapper.GetInterceptor()
		stats["viper_intercepted"] = interceptor.IsIntercepted()
		stats["viper_callbacks"] = interceptor.GetCallbackCount()
	}

	// Goroutine管理统计
	if m.goroutineManager != nil {
		stats["goroutine_interception_enabled"] = m.goroutineManager.IsEnabled()
		if pool := m.goroutineManager.GetSchedulerPool(); pool != nil {
			stats["scheduler_pool_running"] = pool.IsRunning()
			stats["scheduler_pool_workers"] = pool.GetWorkerCount()
			stats["scheduler_pool_queue_size"] = pool.GetCurrentQueueSize()
		}
	}

	// 调度池统计
	if m.schedulerPool != nil {
		poolStats := m.schedulerPool.GetDetailedMetrics()
		stats["scheduler_pool_metrics"] = map[string]interface{}{
			"active_workers":    poolStats.ActiveWorkers,
			"queued_tasks":      poolStats.QueuedTasks,
			"completed_tasks":   poolStats.CompletedTasks,
			"failed_tasks":      poolStats.FailedTasks,
			"rejected_tasks":    poolStats.RejectedTasks,
			"total_submissions": poolStats.TotalSubmissions,
			"success_rate":      poolStats.SuccessRate,
			"average_wait_time": poolStats.AverageWaitTime.String(),
			"average_exec_time": poolStats.AverageExecTime.String(),
			"uptime":            poolStats.Uptime.String(),
		}
	}

	return stats
}
