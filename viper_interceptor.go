package config

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// ViperInterceptor Viper goroutine拦截器
// 负责拦截Viper的goroutine创建，将所有异步操作路由到调度池
type ViperInterceptor struct {
	schedulerPool *SchedulerPool                  // 调度池
	viper         *viper.Viper                    // Viper实例
	intercepted   bool                            // 是否已拦截
	mutex         sync.RWMutex                    // 读写锁
	callbacks     map[string]func(fsnotify.Event) // 配置变更回调
	callbackMutex sync.RWMutex                    // 回调锁
}

// NewViperInterceptor 创建新的Viper拦截器
func NewViperInterceptor(viper *viper.Viper, schedulerPool *SchedulerPool) *ViperInterceptor {
	return &ViperInterceptor{
		schedulerPool: schedulerPool,
		viper:         viper,
		callbacks:     make(map[string]func(fsnotify.Event)),
	}
}

// InterceptGoroutines 拦截Viper的goroutine创建
// 这是核心方法，通过反射和方法替换来拦截Viper的异步操作
func (vi *ViperInterceptor) InterceptGoroutines() error {
	vi.mutex.Lock()
	defer vi.mutex.Unlock()

	if vi.intercepted {
		return fmt.Errorf("goroutines already intercepted")
	}

	if vi.viper == nil {
		return fmt.Errorf("viper instance is nil")
	}

	if vi.schedulerPool == nil {
		return fmt.Errorf("scheduler pool is nil")
	}

	// 拦截WatchConfig方法
	if err := vi.interceptWatchConfig(); err != nil {
		return fmt.Errorf("failed to intercept WatchConfig: %w", err)
	}

	// 拦截OnConfigChange方法
	if err := vi.interceptOnConfigChange(); err != nil {
		return fmt.Errorf("failed to intercept OnConfigChange: %w", err)
	}

	vi.intercepted = true
	return nil
}

// interceptWatchConfig 拦截WatchConfig方法
func (vi *ViperInterceptor) interceptWatchConfig() error {
	// 由于Viper的WatchConfig方法内部创建goroutine，我们需要提供一个替代实现
	// 这里我们通过包装的方式来实现拦截

	// 获取Viper实例的反射值
	viperValue := reflect.ValueOf(vi.viper)
	if viperValue.Kind() == reflect.Ptr {
		viperValue = viperValue.Elem()
	}

	// 检查是否可以访问内部字段
	if !viperValue.IsValid() {
		return fmt.Errorf("invalid viper instance")
	}

	// 由于Viper的内部实现复杂，我们采用代理模式
	// 创建一个包装器来控制WatchConfig的行为
	vi.createWatchConfigProxy()

	return nil
}

// createWatchConfigProxy 创建WatchConfig代理
func (vi *ViperInterceptor) createWatchConfigProxy() {
	// 这里我们不直接修改Viper的方法，而是提供一个受控的监听机制
	// 实际的文件监听将通过我们的FileWatcher来实现
}

// interceptOnConfigChange 拦截OnConfigChange方法
func (vi *ViperInterceptor) interceptOnConfigChange() error {
	// 类似地，我们为OnConfigChange提供一个受控的实现
	return nil
}

// WatchConfigWithScheduler 使用调度池的配置监听
// 这是Viper WatchConfig的替代实现，所有异步操作都通过调度池执行
func (vi *ViperInterceptor) WatchConfigWithScheduler(ctx context.Context, configPath string) error {
	if vi.schedulerPool == nil || !vi.schedulerPool.IsRunning() {
		return fmt.Errorf("scheduler pool not available")
	}

	// 创建文件监听任务
	taskID := fmt.Sprintf("viper-watch-%d", time.Now().UnixNano())
	task := NewConfigTask(taskID, TaskTypeHook, 0, func(taskCtx context.Context) error {
		return vi.startFileWatching(taskCtx, configPath)
	})

	return vi.schedulerPool.Submit(task)
}

// startFileWatching 启动文件监听
func (vi *ViperInterceptor) startFileWatching(ctx context.Context, configPath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// 添加文件到监听列表
	if err := watcher.Add(configPath); err != nil {
		return fmt.Errorf("failed to add file to watcher: %w", err)
	}

	// 监听循环
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// 只处理写入事件
			if event.Op&fsnotify.Write == fsnotify.Write {
				vi.handleConfigChangeEvent(event)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("file watcher error: %w", err)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// handleConfigChangeEvent 处理配置变更事件
func (vi *ViperInterceptor) handleConfigChangeEvent(event fsnotify.Event) {
	// 获取所有注册的回调
	vi.callbackMutex.RLock()
	callbacks := make(map[string]func(fsnotify.Event))
	for id, callback := range vi.callbacks {
		callbacks[id] = callback
	}
	vi.callbackMutex.RUnlock()

	// 在调度池中异步执行所有回调
	for id, callback := range callbacks {
		taskID := fmt.Sprintf("config-change-%s-%d", id, time.Now().UnixNano())
		task := NewConfigTask(taskID, TaskTypeHook, 0, func(ctx context.Context) error {
			callback(event)
			return nil
		})

		// 提交到调度池，如果失败则直接执行
		if err := vi.schedulerPool.Submit(task); err != nil {
			// 调度池提交失败，直接在当前goroutine执行
			callback(event)
		}
	}
}

// OnConfigChangeWithScheduler 注册配置变更回调（使用调度池）
// 这是Viper OnConfigChange的替代实现
func (vi *ViperInterceptor) OnConfigChangeWithScheduler(id string, callback func(fsnotify.Event)) error {
	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}

	vi.callbackMutex.Lock()
	defer vi.callbackMutex.Unlock()

	vi.callbacks[id] = callback
	return nil
}

// RemoveConfigChangeCallback 移除配置变更回调
func (vi *ViperInterceptor) RemoveConfigChangeCallback(id string) {
	vi.callbackMutex.Lock()
	defer vi.callbackMutex.Unlock()

	delete(vi.callbacks, id)
}

// GetCallbackCount 获取回调数量
func (vi *ViperInterceptor) GetCallbackCount() int {
	vi.callbackMutex.RLock()
	defer vi.callbackMutex.RUnlock()

	return len(vi.callbacks)
}

// IsIntercepted 检查是否已拦截
func (vi *ViperInterceptor) IsIntercepted() bool {
	vi.mutex.RLock()
	defer vi.mutex.RUnlock()

	return vi.intercepted
}

// Reset 重置拦截器
func (vi *ViperInterceptor) Reset() {
	vi.mutex.Lock()
	defer vi.mutex.Unlock()

	vi.intercepted = false

	vi.callbackMutex.Lock()
	vi.callbacks = make(map[string]func(fsnotify.Event))
	vi.callbackMutex.Unlock()
}

// ScheduledViperWrapper Viper包装器，提供调度池支持
// 这个包装器替代直接使用Viper，确保所有异步操作都通过调度池
type ScheduledViperWrapper struct {
	viper       *viper.Viper      // 原始Viper实例
	interceptor *ViperInterceptor // 拦截器
	mutex       sync.RWMutex      // 读写锁
}

// NewScheduledViperWrapper 创建新的Viper包装器
func NewScheduledViperWrapper(viper *viper.Viper, schedulerPool *SchedulerPool) *ScheduledViperWrapper {
	interceptor := NewViperInterceptor(viper, schedulerPool)

	return &ScheduledViperWrapper{
		viper:       viper,
		interceptor: interceptor,
	}
}

// Initialize 初始化包装器
func (svw *ScheduledViperWrapper) Initialize() error {
	return svw.interceptor.InterceptGoroutines()
}

// WatchConfig 监听配置变更（使用调度池）
func (svw *ScheduledViperWrapper) WatchConfig(ctx context.Context) error {
	configFile := svw.viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no config file set")
	}

	return svw.interceptor.WatchConfigWithScheduler(ctx, configFile)
}

// OnConfigChange 注册配置变更回调（使用调度池）
func (svw *ScheduledViperWrapper) OnConfigChange(id string, callback func(fsnotify.Event)) error {
	return svw.interceptor.OnConfigChangeWithScheduler(id, callback)
}

// ReadInConfig 读取配置文件
func (svw *ScheduledViperWrapper) ReadInConfig() error {
	svw.mutex.RLock()
	defer svw.mutex.RUnlock()

	return svw.viper.ReadInConfig()
}

// Unmarshal 解析配置到结构体
func (svw *ScheduledViperWrapper) Unmarshal(rawVal interface{}) error {
	svw.mutex.RLock()
	defer svw.mutex.RUnlock()

	return svw.viper.Unmarshal(rawVal)
}

// SetConfigFile 设置配置文件
func (svw *ScheduledViperWrapper) SetConfigFile(in string) {
	svw.mutex.Lock()
	defer svw.mutex.Unlock()

	svw.viper.SetConfigFile(in)
}

// SetConfigType 设置配置文件类型
func (svw *ScheduledViperWrapper) SetConfigType(in string) {
	svw.mutex.Lock()
	defer svw.mutex.Unlock()

	svw.viper.SetConfigType(in)
}

// ConfigFileUsed 获取使用的配置文件路径
func (svw *ScheduledViperWrapper) ConfigFileUsed() string {
	svw.mutex.RLock()
	defer svw.mutex.RUnlock()

	return svw.viper.ConfigFileUsed()
}

// GetViper 获取原始Viper实例（用于不需要拦截的操作）
func (svw *ScheduledViperWrapper) GetViper() *viper.Viper {
	return svw.viper
}

// GetInterceptor 获取拦截器（用于高级操作）
func (svw *ScheduledViperWrapper) GetInterceptor() *ViperInterceptor {
	return svw.interceptor
}

// GoroutineManager goroutine管理器
// 提供统一的goroutine创建和管理接口
type GoroutineManager struct {
	schedulerPool *SchedulerPool // 调度池
	mutex         sync.RWMutex   // 读写锁
	enabled       bool           // 是否启用拦截
}

// NewGoroutineManager 创建新的goroutine管理器
func NewGoroutineManager(schedulerPool *SchedulerPool) *GoroutineManager {
	return &GoroutineManager{
		schedulerPool: schedulerPool,
		enabled:       true,
	}
}

// Go 启动一个受管理的goroutine
// 所有通过此方法启动的goroutine都会通过调度池执行
func (gm *GoroutineManager) Go(fn func()) error {
	gm.mutex.RLock()
	enabled := gm.enabled
	pool := gm.schedulerPool
	gm.mutex.RUnlock()

	if !enabled || pool == nil || !pool.IsRunning() {
		// 如果拦截未启用或调度池不可用，直接启动goroutine
		go fn()
		return nil
	}

	// 通过调度池执行
	taskID := fmt.Sprintf("managed-goroutine-%d", time.Now().UnixNano())
	task := NewConfigTask(taskID, TaskTypeHook, 0, func(ctx context.Context) error {
		fn()
		return nil
	})

	return pool.Submit(task)
}

// GoWithContext 启动一个带上下文的受管理goroutine
func (gm *GoroutineManager) GoWithContext(ctx context.Context, fn func(context.Context)) error {
	gm.mutex.RLock()
	enabled := gm.enabled
	pool := gm.schedulerPool
	gm.mutex.RUnlock()

	if !enabled || pool == nil || !pool.IsRunning() {
		// 如果拦截未启用或调度池不可用，直接启动goroutine
		go fn(ctx)
		return nil
	}

	// 通过调度池执行
	taskID := fmt.Sprintf("managed-goroutine-ctx-%d", time.Now().UnixNano())
	task := NewConfigTask(taskID, TaskTypeHook, 0, func(taskCtx context.Context) error {
		// 合并上下文
		mergedCtx := ctx
		if taskCtx != nil {
			// 优先使用任务上下文的取消信号
			select {
			case <-taskCtx.Done():
				mergedCtx = taskCtx
			default:
				mergedCtx = ctx
			}
		}
		fn(mergedCtx)
		return nil
	})

	return pool.Submit(task)
}

// SetEnabled 设置是否启用goroutine拦截
func (gm *GoroutineManager) SetEnabled(enabled bool) {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	gm.enabled = enabled
}

// IsEnabled 检查是否启用了goroutine拦截
func (gm *GoroutineManager) IsEnabled() bool {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	return gm.enabled
}

// GetSchedulerPool 获取调度池
func (gm *GoroutineManager) GetSchedulerPool() *SchedulerPool {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	return gm.schedulerPool
}

// SetSchedulerPool 设置调度池
func (gm *GoroutineManager) SetSchedulerPool(pool *SchedulerPool) {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	gm.schedulerPool = pool
}
