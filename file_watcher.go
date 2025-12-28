package config

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileChangeHandler 文件变更处理器函数类型
type FileChangeHandler func(event fsnotify.Event) error

// FileChangeHandlerWithID 带ID的文件变更处理器
type FileChangeHandlerWithID struct {
	ID       string            // 处理器唯一标识
	Handler  FileChangeHandler // 处理器函数
	Priority int               // 优先级，数值越小优先级越高
	Async    bool              // 是否异步执行
	Timeout  time.Duration     // 执行超时时间
}

// NewFileChangeHandler 创建新的文件变更处理器
func NewFileChangeHandler(id string, handler FileChangeHandler) *FileChangeHandlerWithID {
	return &FileChangeHandlerWithID{
		ID:       id,
		Handler:  handler,
		Priority: 0,
		Async:    true,             // 默认异步执行
		Timeout:  30 * time.Second, // 默认30秒超时
	}
}

// SetPriority 设置处理器优先级
func (h *FileChangeHandlerWithID) SetPriority(priority int) *FileChangeHandlerWithID {
	h.Priority = priority
	return h
}

// SetAsync 设置是否异步执行
func (h *FileChangeHandlerWithID) SetAsync(async bool) *FileChangeHandlerWithID {
	h.Async = async
	return h
}

// SetTimeout 设置执行超时时间
func (h *FileChangeHandlerWithID) SetTimeout(timeout time.Duration) *FileChangeHandlerWithID {
	h.Timeout = timeout
	return h
}

// FileWatcherStatus 文件监听器状态
type FileWatcherStatus int32

const (
	StatusStopped FileWatcherStatus = iota
	StatusStarting
	StatusRunning
	StatusStopping
)

// String 返回状态的字符串表示
func (s FileWatcherStatus) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// FileWatcher 文件监听器
//
// FileWatcher 是一个高级的文件变更监听器，基于fsnotify实现，
// 提供了防抖、异步处理、优雅关闭等高级功能。
//
// 核心特性：
// - 防抖处理：避免短时间内重复触发
// - 异步执行：支持同步和异步两种处理模式
// - 优先级排序：处理器按优先级顺序执行
// - 调度池集成：异步任务通过调度池执行
// - 状态管理：提供详细的运行状态信息
// - 优雅关闭：确保正在处理的事件完成后再退出
//
// 生命周期：
// 1. 创建：使用NewFileWatcher创建实例
// 2. 配置：添加文件变更处理器
// 3. 启动：调用Start方法开始监听
// 4. 运行：自动处理文件变更事件
// 5. 停止：调用Stop方法停止监听
//
// 使用示例：
//
//	watcher := NewFileWatcher("/path/to/config.yaml", debouncer, pool)
//	watcher.AddHandlerWithID(NewFileChangeHandler("reload", reloadHandler))
//	err := watcher.Start(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer watcher.Stop(5 * time.Second)
type FileWatcher struct {
	// 配置
	filepath      string         // 监听的文件路径
	debouncer     *Debouncer     // 防抖器
	schedulerPool *SchedulerPool // 调度池

	// 处理器
	handlers      []*FileChangeHandlerWithID // 文件变更处理器列表
	handlersMutex sync.RWMutex               // 处理器列表的读写锁

	// 状态管理
	status  int32              // 原子状态变量
	ctx     context.Context    // 上下文
	cancel  context.CancelFunc // 取消函数
	watcher *fsnotify.Watcher  // fsnotify监听器

	// 同步
	mutex sync.RWMutex   // 主要的读写锁
	wg    sync.WaitGroup // 等待组

	// 统计信息
	lastChange time.Time // 最后变更时间
	eventCount int64     // 事件计数
	errorCount int64     // 错误计数
	startTime  time.Time // 启动时间
}

// NewFileWatcher 创建新的文件监听器
//
// 创建一个新的FileWatcher实例，用于监听指定文件的变更事件。
//
// 参数：
//
//	filepath: 要监听的文件路径，必须是有效的文件路径
//	debouncer: 防抖器实例，如果为nil则使用默认100ms防抖
//	pool: 调度池实例，用于异步执行处理器，可以为nil
//
// 返回值：
//
//	*FileWatcher: 新创建的文件监听器实例
//
// 默认配置：
// - 初始状态：StatusStopped
// - 防抖时间：100ms（如果debouncer为nil）
// - 处理器列表：空
//
// 注意：
// - 如果调度池为nil，异步处理器将在独立的goroutine中执行
// - 创建后需要调用Start方法才能开始监听
// - 建议在使用前添加至少一个文件变更处理器
func NewFileWatcher(filepath string, debouncer *Debouncer, pool *SchedulerPool) *FileWatcher {
	if debouncer == nil {
		debouncer = NewDebouncer(100 * time.Millisecond) // 默认100ms防抖
	}

	return &FileWatcher{
		filepath:      filepath,
		debouncer:     debouncer,
		schedulerPool: pool,
		handlers:      make([]*FileChangeHandlerWithID, 0),
		status:        int32(StatusStopped),
	}
}

// Start 启动文件监听
func (w *FileWatcher) Start(ctx context.Context) error {
	// 原子性检查和设置状态
	if !atomic.CompareAndSwapInt32(&w.status, int32(StatusStopped), int32(StatusStarting)) {
		currentStatus := FileWatcherStatus(atomic.LoadInt32(&w.status))
		if currentStatus == StatusStopping {
			return ErrWatcherNotRunning // 正在停止中，拒绝启动请求
		}
		return fmt.Errorf("file watcher is not in stopped state, current status: %s", currentStatus.String())
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 验证文件路径
	if w.filepath == "" {
		atomic.StoreInt32(&w.status, int32(StatusStopped))
		return ErrInvalidFilepath
	}

	// 创建fsnotify监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		atomic.StoreInt32(&w.status, int32(StatusStopped))
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// 添加文件到监听列表
	if err := watcher.Add(w.filepath); err != nil {
		watcher.Close()
		atomic.StoreInt32(&w.status, int32(StatusStopped))
		return fmt.Errorf("failed to add file to watcher: %w", err)
	}

	// 设置上下文
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.watcher = watcher
	w.startTime = time.Now()

	// 启动监听goroutine
	w.wg.Add(1)
	go w.watchLoop()

	// 设置为运行状态
	atomic.StoreInt32(&w.status, int32(StatusRunning))

	return nil
}

// Stop 停止文件监听
func (w *FileWatcher) Stop(timeout time.Duration) error {
	// 原子性检查和设置状态
	currentStatus := atomic.LoadInt32(&w.status)
	if currentStatus == int32(StatusStopped) || currentStatus == int32(StatusStopping) {
		return nil // 已经停止或正在停止
	}

	if !atomic.CompareAndSwapInt32(&w.status, currentStatus, int32(StatusStopping)) {
		return fmt.Errorf("failed to change status to stopping")
	}

	return w.performStop(timeout)
}

// performStop 执行停止操作
func (w *FileWatcher) performStop(timeout time.Duration) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 取消上下文，通知所有goroutine停止
	if w.cancel != nil {
		w.cancel()
	}

	// 创建超时上下文用于优雅关闭
	stopCtx, stopCancel := context.WithTimeout(context.Background(), timeout)
	defer stopCancel()

	// 等待所有goroutine完成
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	var stopErr error
	select {
	case <-done:
		// 正常完成，所有goroutine已退出
	case <-stopCtx.Done():
		// 超时，记录错误但继续清理
		stopErr = ErrWatcherStopTimeout
	}

	// 执行资源清理
	w.cleanup()

	// 设置为停止状态
	atomic.StoreInt32(&w.status, int32(StatusStopped))

	return stopErr
}

// cleanup 清理资源
func (w *FileWatcher) cleanup() {
	// 关闭fsnotify监听器
	if w.watcher != nil {
		w.watcher.Close()
		w.watcher = nil
	}

	// 停止防抖器
	if w.debouncer != nil {
		w.debouncer.Stop()
	}

	// 清理上下文
	w.ctx = nil
	w.cancel = nil

	// 重置统计信息（可选，根据需求决定是否保留）
	// atomic.StoreInt64(&w.eventCount, 0)
	// atomic.StoreInt64(&w.errorCount, 0)
}

// Restart 重启文件监听器
func (w *FileWatcher) Restart(ctx context.Context, timeout time.Duration) error {
	// 如果正在运行，先停止
	if w.IsRunning() {
		if err := w.Stop(timeout); err != nil {
			return fmt.Errorf("failed to stop watcher before restart: %w", err)
		}
	}

	// 等待完全停止
	maxWait := timeout
	if maxWait <= 0 {
		maxWait = 5 * time.Second // 默认等待5秒
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), maxWait)
	defer waitCancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if w.GetStatus() == StatusStopped {
				// 已完全停止，可以重启
				return w.Start(ctx)
			}
		case <-waitCtx.Done():
			return fmt.Errorf("timeout waiting for watcher to stop before restart")
		}
	}
}

// ForceStop 强制停止文件监听器（不等待优雅关闭）
func (w *FileWatcher) ForceStop() error {
	// 直接设置状态为停止中
	atomic.StoreInt32(&w.status, int32(StatusStopping))

	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 立即取消上下文
	if w.cancel != nil {
		w.cancel()
	}

	// 执行资源清理
	w.cleanup()

	// 设置为停止状态
	atomic.StoreInt32(&w.status, int32(StatusStopped))

	return nil
}

// watchLoop 监听循环
func (w *FileWatcher) watchLoop() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return // 监听器已关闭
			}

			// 过滤事件类型，只处理写入事件
			if event.Op&fsnotify.Write == fsnotify.Write {
				w.handleFileEvent(event)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return // 监听器已关闭
			}

			atomic.AddInt64(&w.errorCount, 1)
			// 可以在这里添加错误处理逻辑，比如记录日志
			fmt.Printf("File watcher error: %v\n", err)

		case <-w.ctx.Done():
			return // 上下文取消
		}
	}
}

// handleFileEvent 处理文件事件
func (w *FileWatcher) handleFileEvent(event fsnotify.Event) {
	// 使用防抖器处理事件
	w.debouncer.Debounce(func() {
		w.processFileEvent(event)
	})
}

// processFileEvent 处理文件事件（防抖后）
func (w *FileWatcher) processFileEvent(event fsnotify.Event) {
	atomic.AddInt64(&w.eventCount, 1)
	w.mutex.Lock()
	w.lastChange = time.Now()
	w.mutex.Unlock()

	// 获取处理器列表的副本并按优先级排序
	w.handlersMutex.RLock()
	handlers := make([]*FileChangeHandlerWithID, len(w.handlers))
	copy(handlers, w.handlers)
	w.handlersMutex.RUnlock()

	// 按优先级排序（数值越小优先级越高）
	for i := 0; i < len(handlers)-1; i++ {
		for j := i + 1; j < len(handlers); j++ {
			if handlers[i].Priority > handlers[j].Priority {
				handlers[i], handlers[j] = handlers[j], handlers[i]
			}
		}
	}

	// 分别处理同步和异步处理器
	var syncHandlers []*FileChangeHandlerWithID
	var asyncHandlers []*FileChangeHandlerWithID

	for _, handler := range handlers {
		if handler.Async {
			asyncHandlers = append(asyncHandlers, handler)
		} else {
			syncHandlers = append(syncHandlers, handler)
		}
	}

	// 先执行同步处理器
	for _, handler := range syncHandlers {
		if err := w.executeSyncHandler(handler, event); err != nil {
			atomic.AddInt64(&w.errorCount, 1)
			fmt.Printf("Sync file event handler %s error: %v\n", handler.ID, err)
		}
	}

	// 然后提交异步处理器到调度池
	for _, handler := range asyncHandlers {
		if err := w.executeAsyncHandler(handler, event); err != nil {
			atomic.AddInt64(&w.errorCount, 1)
			fmt.Printf("Failed to submit async file event handler %s: %v\n", handler.ID, err)
		}
	}
}

// executeSyncHandler 执行同步处理器
func (w *FileWatcher) executeSyncHandler(handler *FileChangeHandlerWithID, event fsnotify.Event) error {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), handler.Timeout)
	defer cancel()

	// 在goroutine中执行以支持超时
	done := make(chan error, 1)
	go func() {
		done <- handler.Handler(event)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("handler %s execution timeout", handler.ID)
	}
}

// executeAsyncHandler 执行异步处理器
func (w *FileWatcher) executeAsyncHandler(handler *FileChangeHandlerWithID, event fsnotify.Event) error {
	// 如果有调度池，使用调度池执行
	if w.schedulerPool != nil && w.schedulerPool.IsRunning() {
		taskID := fmt.Sprintf("file-event-%s-%d", handler.ID, time.Now().UnixNano())
		task := NewConfigTask(taskID, TaskTypeHook, handler.Priority, func(ctx context.Context) error {
			// 创建带超时的上下文
			timeoutCtx, cancel := context.WithTimeout(ctx, handler.Timeout)
			defer cancel()

			// 在goroutine中执行以支持超时
			done := make(chan error, 1)
			go func() {
				done <- handler.Handler(event)
			}()

			select {
			case err := <-done:
				return err
			case <-timeoutCtx.Done():
				return fmt.Errorf("handler %s execution timeout", handler.ID)
			}
		})

		return w.schedulerPool.Submit(task)
	} else {
		// 直接在新的goroutine中执行
		go func() {
			if err := w.executeSyncHandler(handler, event); err != nil {
				atomic.AddInt64(&w.errorCount, 1)
				fmt.Printf("Async file event handler %s error: %v\n", handler.ID, err)
			}
		}()
		return nil
	}
}

// AddHandler 添加文件变更处理器（简单版本，兼容旧API）
func (w *FileWatcher) AddHandler(handler FileChangeHandler) {
	if handler == nil {
		return
	}

	// 生成唯一ID
	id := fmt.Sprintf("handler-%d", time.Now().UnixNano())
	handlerWithID := NewFileChangeHandler(id, handler)

	w.AddHandlerWithID(handlerWithID)
}

// AddHandlerWithID 添加带ID的文件变更处理器
func (w *FileWatcher) AddHandlerWithID(handler *FileChangeHandlerWithID) error {
	if handler == nil || handler.Handler == nil {
		return ErrInvalidFileHandler
	}

	w.handlersMutex.Lock()
	defer w.handlersMutex.Unlock()

	// 检查ID是否已存在
	for _, h := range w.handlers {
		if h.ID == handler.ID {
			return fmt.Errorf("handler with ID %s already exists", handler.ID)
		}
	}

	w.handlers = append(w.handlers, handler)
	return nil
}

// RemoveHandler 移除文件变更处理器（简单版本，移除第一个匹配的）
func (w *FileWatcher) RemoveHandler(handler FileChangeHandler) {
	if handler == nil {
		return
	}

	w.handlersMutex.Lock()
	defer w.handlersMutex.Unlock()

	// 由于函数无法直接比较，这里使用简单的实现
	// 在实际使用中，建议使用RemoveHandlerByID
	for i := len(w.handlers) - 1; i >= 0; i-- {
		w.handlers = append(w.handlers[:i], w.handlers[i+1:]...)
		break // 只移除第一个匹配的处理器
	}
}

// RemoveHandlerByID 根据ID移除文件变更处理器
func (w *FileWatcher) RemoveHandlerByID(id string) bool {
	if id == "" {
		return false
	}

	w.handlersMutex.Lock()
	defer w.handlersMutex.Unlock()

	for i, handler := range w.handlers {
		if handler.ID == id {
			w.handlers = append(w.handlers[:i], w.handlers[i+1:]...)
			return true
		}
	}
	return false
}

// GetHandlerByID 根据ID获取处理器
func (w *FileWatcher) GetHandlerByID(id string) *FileChangeHandlerWithID {
	if id == "" {
		return nil
	}

	w.handlersMutex.RLock()
	defer w.handlersMutex.RUnlock()

	for _, handler := range w.handlers {
		if handler.ID == id {
			return handler
		}
	}
	return nil
}

// ListHandlers 列出所有处理器的ID和优先级
func (w *FileWatcher) ListHandlers() []map[string]interface{} {
	w.handlersMutex.RLock()
	defer w.handlersMutex.RUnlock()

	result := make([]map[string]interface{}, len(w.handlers))
	for i, handler := range w.handlers {
		result[i] = map[string]interface{}{
			"id":       handler.ID,
			"priority": handler.Priority,
			"async":    handler.Async,
			"timeout":  handler.Timeout.String(),
		}
	}
	return result
}

// ClearHandlers 清除所有处理器
func (w *FileWatcher) ClearHandlers() {
	w.handlersMutex.Lock()
	defer w.handlersMutex.Unlock()

	w.handlers = w.handlers[:0]
}

// GetHandlerCount 获取处理器数量
func (w *FileWatcher) GetHandlerCount() int {
	w.handlersMutex.RLock()
	defer w.handlersMutex.RUnlock()

	return len(w.handlers)
}

// SetSchedulerPool 设置调度池
func (w *FileWatcher) SetSchedulerPool(pool *SchedulerPool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.schedulerPool = pool
}

// GetSchedulerPool 获取调度池
func (w *FileWatcher) GetSchedulerPool() *SchedulerPool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.schedulerPool
}

// IsRunning 检查是否正在运行
func (w *FileWatcher) IsRunning() bool {
	status := atomic.LoadInt32(&w.status)
	return status == int32(StatusRunning)
}

// GetStatus 获取当前状态
func (w *FileWatcher) GetStatus() FileWatcherStatus {
	return FileWatcherStatus(atomic.LoadInt32(&w.status))
}

// GetFilepath 获取监听的文件路径
func (w *FileWatcher) GetFilepath() string {
	return w.filepath
}

// GetLastChangeTime 获取最后变更时间
func (w *FileWatcher) GetLastChangeTime() time.Time {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.lastChange
}

// GetEventCount 获取事件计数
func (w *FileWatcher) GetEventCount() int64 {
	return atomic.LoadInt64(&w.eventCount)
}

// GetErrorCount 获取错误计数
func (w *FileWatcher) GetErrorCount() int64 {
	return atomic.LoadInt64(&w.errorCount)
}

// GetUptime 获取运行时间
func (w *FileWatcher) GetUptime() time.Duration {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if w.startTime.IsZero() {
		return 0
	}
	return time.Since(w.startTime)
}

// GetStats 获取统计信息
func (w *FileWatcher) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"status":        w.GetStatus().String(),
		"filepath":      w.GetFilepath(),
		"running":       w.IsRunning(),
		"uptime":        w.GetUptime().String(),
		"event_count":   w.GetEventCount(),
		"error_count":   w.GetErrorCount(),
		"handler_count": w.GetHandlerCount(),
		"last_change":   w.GetLastChangeTime(),
		"handlers":      w.ListHandlers(),
	}

	// 添加调度池信息
	if w.schedulerPool != nil {
		stats["scheduler_pool"] = map[string]interface{}{
			"running":        w.schedulerPool.IsRunning(),
			"worker_count":   w.schedulerPool.GetWorkerCount(),
			"queue_size":     w.schedulerPool.GetCurrentQueueSize(),
			"queue_capacity": w.schedulerPool.GetQueueCapacity(),
		}
	} else {
		stats["scheduler_pool"] = nil
	}

	return stats
}

// Debouncer 防抖器
type Debouncer struct {
	duration time.Duration // 防抖时间间隔
	timer    *time.Timer   // 定时器
	mutex    sync.Mutex    // 互斥锁
	lastCall time.Time     // 最后调用时间
}

// NewDebouncer 创建防抖器
func NewDebouncer(duration time.Duration) *Debouncer {
	return &Debouncer{
		duration: duration,
	}
}

// Debounce 防抖执行
func (d *Debouncer) Debounce(fn func()) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 记录当前调用时间
	now := time.Now()
	d.lastCall = now

	// 如果已有定时器，停止它
	if d.timer != nil {
		d.timer.Stop()
	}

	// 创建新的定时器
	d.timer = time.AfterFunc(d.duration, func() {
		d.mutex.Lock()
		defer d.mutex.Unlock()

		// 检查是否是最后一次调用
		if time.Since(d.lastCall) >= d.duration {
			fn()
		}
	})
}

// SetDuration 设置防抖时间间隔
func (d *Debouncer) SetDuration(duration time.Duration) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.duration = duration
}

// GetDuration 获取防抖时间间隔
func (d *Debouncer) GetDuration() time.Duration {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.duration
}

// Stop 停止防抖器
func (d *Debouncer) Stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
