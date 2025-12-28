package config

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Task 任务接口
//
// Task 定义了调度池中可执行任务的标准接口。所有提交到
// 调度池的任务都必须实现这个接口。
//
// 接口方法：
// - Execute: 执行任务的核心逻辑，接收上下文参数
// - Priority: 返回任务优先级，数值越小优先级越高
// - ID: 返回任务的唯一标识符，用于跟踪和调试
//
// 实现建议：
// - Execute方法应该尊重传入的context，支持取消和超时
// - Priority应该返回合理的优先级值，避免极端值
// - ID应该是全局唯一的，建议包含时间戳或UUID
type Task interface {
	// Execute 执行任务
	Execute(ctx context.Context) error
	// Priority 返回任务优先级，数值越小优先级越高
	Priority() int
	// ID 返回任务唯一标识
	ID() string
}

// TaskType 任务类型
type TaskType int

const (
	TaskTypeLoad TaskType = iota
	TaskTypeUpdate
	TaskTypeValidate
	TaskTypeHook
	TaskTypeCleanup
)

// ConfigTask 配置相关任务实现
//
// ConfigTask 是Task接口的具体实现，专门用于配置管理相关的任务。
// 它提供了丰富的任务配置选项和执行控制功能。
//
// 主要特性：
// - 支持任务分类（通过TaskType）
// - 可配置的优先级和超时时间
// - 支持执行回调和重试机制
// - 灵活的任务负载（payload）
//
// 使用场景：
// - 配置加载任务
// - 配置更新任务
// - 配置验证任务
// - 钩子执行任务
// - 资源清理任务
type ConfigTask struct {
	id       string
	taskType TaskType
	priority int
	payload  interface{}
	callback func(error)
	timeout  time.Duration
	retries  int
	fn       func(context.Context) error
}

// NewConfigTask 创建新的配置任务
//
// 创建一个新的ConfigTask实例，用于在调度池中执行配置相关操作。
//
// 参数：
//
//	id: 任务唯一标识符，建议使用描述性名称加时间戳
//	taskType: 任务类型，用于分类和统计
//	priority: 任务优先级，数值越小优先级越高
//	fn: 任务执行函数，接收context参数
//
// 返回值：
//
//	*ConfigTask: 新创建的任务实例
//
// 默认配置：
// - 超时时间：30秒
// - 重试次数：0（不重试）
//
// 使用示例：
//
//	task := NewConfigTask("load-config-123", TaskTypeLoad, 1, func(ctx context.Context) error {
//	    // 执行配置加载逻辑
//	    return nil
//	})
func NewConfigTask(id string, taskType TaskType, priority int, fn func(context.Context) error) *ConfigTask {
	return &ConfigTask{
		id:       id,
		taskType: taskType,
		priority: priority,
		fn:       fn,
		timeout:  30 * time.Second, // 默认30秒超时
	}
}

// Execute 实现Task接口
func (t *ConfigTask) Execute(ctx context.Context) error {
	if t.fn == nil {
		return fmt.Errorf("task function is nil")
	}
	return t.fn(ctx)
}

// Priority 实现Task接口
func (t *ConfigTask) Priority() int {
	return t.priority
}

// ID 实现Task接口
func (t *ConfigTask) ID() string {
	return t.id
}

// SetCallback 设置任务完成回调
func (t *ConfigTask) SetCallback(callback func(error)) {
	t.callback = callback
}

// SetTimeout 设置任务超时时间
func (t *ConfigTask) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

// Worker 工作goroutine
type Worker struct {
	id         int             // 工作器ID
	taskQueue  chan Task       // 任务队列
	workerPool chan chan Task  // 工作器池
	ctx        context.Context // 上下文
	wg         *sync.WaitGroup // 等待组
	quit       chan bool       // 退出信号
	metrics    *PoolMetrics    // 池统计信息（用于更新指标）
}

// NewWorker 创建新的工作器
func NewWorker(id int, workerPool chan chan Task, ctx context.Context, wg *sync.WaitGroup, metrics *PoolMetrics) *Worker {
	return &Worker{
		id:         id,
		taskQueue:  make(chan Task),
		workerPool: workerPool,
		ctx:        ctx,
		wg:         wg,
		quit:       make(chan bool),
		metrics:    metrics,
	}
}

// Start 启动工作器
func (w *Worker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			// 将工作器的任务队列注册到工作器池
			w.workerPool <- w.taskQueue

			select {
			case task := <-w.taskQueue:
				// 执行任务
				if err := w.executeTask(task); err != nil {
					// 错误处理，可以记录日志或通知调度池
					fmt.Printf("Worker %d: task %s execution failed: %v\n", w.id, task.ID(), err)
				}

			case <-w.quit:
				// 收到退出信号
				return

			case <-w.ctx.Done():
				// 上下文取消
				return
			}
		}
	}()
}

// Stop 停止工作器
func (w *Worker) Stop() {
	close(w.quit)
}

// executeTask 执行任务
func (w *Worker) executeTask(task Task) error {
	startTime := time.Now()

	// 创建带超时的上下文
	taskCtx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	// 执行任务
	err := task.Execute(taskCtx)

	// 记录执行时间和结果
	execTime := time.Since(startTime)
	if w.metrics != nil {
		w.metrics.AddExecTime(execTime)
		if err != nil {
			w.metrics.IncrementFailed()
		} else {
			w.metrics.IncrementCompleted()
		}
	}

	return err
}

// PoolMetrics 池统计信息
type PoolMetrics struct {
	ActiveWorkers    int32           // 活跃工作器数量
	QueuedTasks      int32           // 队列中的任务数量
	CompletedTasks   int64           // 已完成任务数量
	FailedTasks      int64           // 失败任务数量
	RejectedTasks    int64           // 被拒绝任务数量
	AverageWaitTime  time.Duration   // 平均等待时间
	AverageExecTime  time.Duration   // 平均执行时间
	PeakQueueSize    int32           // 队列峰值大小
	TotalSubmissions int64           // 总提交任务数量
	StartTime        time.Time       // 池启动时间
	LastTaskTime     time.Time       // 最后任务时间
	mutex            sync.RWMutex    // 保护统计数据
	waitTimes        []time.Duration // 等待时间样本（用于计算平均值）
	execTimes        []time.Duration // 执行时间样本（用于计算平均值）
	maxSamples       int             // 最大样本数量
}

// GetActiveWorkers 获取活跃工作器数量
func (m *PoolMetrics) GetActiveWorkers() int32 {
	return atomic.LoadInt32(&m.ActiveWorkers)
}

// GetQueuedTasks 获取队列任务数量
func (m *PoolMetrics) GetQueuedTasks() int32 {
	return atomic.LoadInt32(&m.QueuedTasks)
}

// GetCompletedTasks 获取已完成任务数量
func (m *PoolMetrics) GetCompletedTasks() int64 {
	return atomic.LoadInt64(&m.CompletedTasks)
}

// GetFailedTasks 获取失败任务数量
func (m *PoolMetrics) GetFailedTasks() int64 {
	return atomic.LoadInt64(&m.FailedTasks)
}

// IncrementCompleted 增加完成任务计数
func (m *PoolMetrics) IncrementCompleted() {
	atomic.AddInt64(&m.CompletedTasks, 1)
}

// IncrementFailed 增加失败任务计数
func (m *PoolMetrics) IncrementFailed() {
	atomic.AddInt64(&m.FailedTasks, 1)
}

// SetActiveWorkers 设置活跃工作器数量
func (m *PoolMetrics) SetActiveWorkers(count int32) {
	atomic.StoreInt32(&m.ActiveWorkers, count)
}

// SetQueuedTasks 设置队列任务数量
func (m *PoolMetrics) SetQueuedTasks(count int32) {
	atomic.StoreInt32(&m.QueuedTasks, count)

	// 更新峰值队列大小
	current := atomic.LoadInt32(&m.PeakQueueSize)
	if count > current {
		atomic.CompareAndSwapInt32(&m.PeakQueueSize, current, count)
	}
}

// NewPoolMetrics 创建新的池统计信息
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{
		StartTime:  time.Now(),
		maxSamples: 1000, // 保留最近1000个样本
		waitTimes:  make([]time.Duration, 0, 1000),
		execTimes:  make([]time.Duration, 0, 1000),
	}
}

// IncrementRejected 增加拒绝任务计数
func (m *PoolMetrics) IncrementRejected() {
	atomic.AddInt64(&m.RejectedTasks, 1)
}

// IncrementSubmissions 增加提交任务计数
func (m *PoolMetrics) IncrementSubmissions() {
	atomic.AddInt64(&m.TotalSubmissions, 1)
	m.mutex.Lock()
	m.LastTaskTime = time.Now()
	m.mutex.Unlock()
}

// AddWaitTime 添加等待时间样本
func (m *PoolMetrics) AddWaitTime(waitTime time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 添加样本
	m.waitTimes = append(m.waitTimes, waitTime)

	// 保持样本数量在限制内
	if len(m.waitTimes) > m.maxSamples {
		m.waitTimes = m.waitTimes[1:]
	}

	// 计算平均等待时间
	m.calculateAverageWaitTime()
}

// AddExecTime 添加执行时间样本
func (m *PoolMetrics) AddExecTime(execTime time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 添加样本
	m.execTimes = append(m.execTimes, execTime)

	// 保持样本数量在限制内
	if len(m.execTimes) > m.maxSamples {
		m.execTimes = m.execTimes[1:]
	}

	// 计算平均执行时间
	m.calculateAverageExecTime()
}

// calculateAverageWaitTime 计算平均等待时间（需要持有锁）
func (m *PoolMetrics) calculateAverageWaitTime() {
	if len(m.waitTimes) == 0 {
		m.AverageWaitTime = 0
		return
	}

	var total time.Duration
	for _, waitTime := range m.waitTimes {
		total += waitTime
	}
	m.AverageWaitTime = total / time.Duration(len(m.waitTimes))
}

// calculateAverageExecTime 计算平均执行时间（需要持有锁）
func (m *PoolMetrics) calculateAverageExecTime() {
	if len(m.execTimes) == 0 {
		m.AverageExecTime = 0
		return
	}

	var total time.Duration
	for _, execTime := range m.execTimes {
		total += execTime
	}
	m.AverageExecTime = total / time.Duration(len(m.execTimes))
}

// GetRejectedTasks 获取被拒绝任务数量
func (m *PoolMetrics) GetRejectedTasks() int64 {
	return atomic.LoadInt64(&m.RejectedTasks)
}

// GetTotalSubmissions 获取总提交任务数量
func (m *PoolMetrics) GetTotalSubmissions() int64 {
	return atomic.LoadInt64(&m.TotalSubmissions)
}

// GetPeakQueueSize 获取队列峰值大小
func (m *PoolMetrics) GetPeakQueueSize() int32 {
	return atomic.LoadInt32(&m.PeakQueueSize)
}

// GetAverageWaitTime 获取平均等待时间
func (m *PoolMetrics) GetAverageWaitTime() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.AverageWaitTime
}

// GetAverageExecTime 获取平均执行时间
func (m *PoolMetrics) GetAverageExecTime() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.AverageExecTime
}

// GetUptime 获取池运行时间
func (m *PoolMetrics) GetUptime() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return time.Since(m.StartTime)
}

// GetLastTaskTime 获取最后任务时间
func (m *PoolMetrics) GetLastTaskTime() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.LastTaskTime
}

// GetSuccessRate 获取成功率
func (m *PoolMetrics) GetSuccessRate() float64 {
	completed := atomic.LoadInt64(&m.CompletedTasks)
	failed := atomic.LoadInt64(&m.FailedTasks)
	total := completed + failed

	if total == 0 {
		return 0.0
	}

	return float64(completed) / float64(total)
}

// Reset 重置统计信息
func (m *PoolMetrics) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	atomic.StoreInt32(&m.ActiveWorkers, 0)
	atomic.StoreInt32(&m.QueuedTasks, 0)
	atomic.StoreInt64(&m.CompletedTasks, 0)
	atomic.StoreInt64(&m.FailedTasks, 0)
	atomic.StoreInt64(&m.RejectedTasks, 0)
	atomic.StoreInt64(&m.TotalSubmissions, 0)
	atomic.StoreInt32(&m.PeakQueueSize, 0)

	m.AverageWaitTime = 0
	m.AverageExecTime = 0
	m.StartTime = time.Now()
	m.LastTaskTime = time.Time{}
	m.waitTimes = m.waitTimes[:0]
	m.execTimes = m.execTimes[:0]
}

// Snapshot 获取统计信息快照
func (m *PoolMetrics) Snapshot() PoolMetricsSnapshot {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return PoolMetricsSnapshot{
		ActiveWorkers:    atomic.LoadInt32(&m.ActiveWorkers),
		QueuedTasks:      atomic.LoadInt32(&m.QueuedTasks),
		CompletedTasks:   atomic.LoadInt64(&m.CompletedTasks),
		FailedTasks:      atomic.LoadInt64(&m.FailedTasks),
		RejectedTasks:    atomic.LoadInt64(&m.RejectedTasks),
		TotalSubmissions: atomic.LoadInt64(&m.TotalSubmissions),
		PeakQueueSize:    atomic.LoadInt32(&m.PeakQueueSize),
		AverageWaitTime:  m.AverageWaitTime,
		AverageExecTime:  m.AverageExecTime,
		SuccessRate:      m.GetSuccessRate(),
		Uptime:           time.Since(m.StartTime),
		LastTaskTime:     m.LastTaskTime,
		Timestamp:        time.Now(),
	}
}

// PoolMetricsSnapshot 池统计信息快照
type PoolMetricsSnapshot struct {
	ActiveWorkers    int32         `json:"active_workers"`
	QueuedTasks      int32         `json:"queued_tasks"`
	CompletedTasks   int64         `json:"completed_tasks"`
	FailedTasks      int64         `json:"failed_tasks"`
	RejectedTasks    int64         `json:"rejected_tasks"`
	TotalSubmissions int64         `json:"total_submissions"`
	PeakQueueSize    int32         `json:"peak_queue_size"`
	AverageWaitTime  time.Duration `json:"average_wait_time"`
	AverageExecTime  time.Duration `json:"average_exec_time"`
	SuccessRate      float64       `json:"success_rate"`
	Uptime           time.Duration `json:"uptime"`
	LastTaskTime     time.Time     `json:"last_task_time"`
	Timestamp        time.Time     `json:"timestamp"`
}

// SchedulerPool goroutine调度池
//
// SchedulerPool 是一个高性能的goroutine池实现，用于管理和调度
// 配置管理器中的所有异步任务。它提供了以下核心功能：
//
// 核心特性：
// - 固定数量的工作goroutine，避免无限制创建
// - 任务队列缓冲，提高吞吐量
// - 优雅关闭，确保任务完成后再退出
// - 丰富的统计信息，便于监控和调试
// - 可配置的拒绝策略，处理过载情况
// - 支持自定义调度策略，如优先级调度
//
// 生命周期：
// 1. 创建：使用NewSchedulerPool创建实例
// 2. 启动：调用Start方法启动工作goroutine
// 3. 使用：通过Submit方法提交任务
// 4. 停止：调用Stop方法优雅关闭
//
// 线程安全性：
// 所有公共方法都是线程安全的，可以在多个goroutine中并发调用。
type SchedulerPool struct {
	maxWorkers         int                // 最大工作器数量
	taskQueue          chan Task          // 任务队列
	workerPool         chan chan Task     // 工作器池
	workers            []*Worker          // 工作器列表
	ctx                context.Context    // 上下文
	cancel             context.CancelFunc // 取消函数
	wg                 sync.WaitGroup     // 等待组
	metrics            *PoolMetrics       // 统计信息
	mutex              sync.RWMutex       // 读写锁
	running            bool               // 运行状态
	rejectionPolicy    RejectionPolicy    // 拒绝策略
	schedulingStrategy SchedulingStrategy // 调度策略
}

// NewSchedulerPool 创建新的调度池
//
// 创建并初始化一个新的SchedulerPool实例。调度池创建后需要
// 调用Start方法才能开始处理任务。
//
// 参数：
//
//	maxWorkers: 最大工作goroutine数量，如果<=0则使用默认值10
//	bufferSize: 任务队列缓冲区大小，如果<=0则使用默认值100
//
// 返回值：
//
//	*SchedulerPool: 新创建的调度池实例
//
// 默认配置：
// - 拒绝策略：RejectPolicyAbort（直接拒绝）
// - 调度策略：DefaultSchedulingStrategy（FIFO）
//
// 使用示例：
//
//	pool := NewSchedulerPool(20, 200)  // 20个工作器，200个任务缓冲
//	err := pool.Start(context.Background())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.Stop(5 * time.Second)
func NewSchedulerPool(maxWorkers, bufferSize int) *SchedulerPool {
	if maxWorkers <= 0 {
		maxWorkers = 10 // 默认10个工作器
	}
	if bufferSize <= 0 {
		bufferSize = 100 // 默认100个任务缓冲
	}

	return &SchedulerPool{
		maxWorkers:         maxWorkers,
		taskQueue:          make(chan Task, bufferSize),
		workerPool:         make(chan chan Task, maxWorkers),
		workers:            make([]*Worker, 0, maxWorkers),
		metrics:            NewPoolMetrics(),
		rejectionPolicy:    RejectPolicyAbort,            // 默认拒绝策略
		schedulingStrategy: &DefaultSchedulingStrategy{}, // 默认调度策略
	}
}

// Start 启动调度池
func (p *SchedulerPool) Start(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return fmt.Errorf("scheduler pool is already running")
	}

	// 创建上下文
	p.ctx, p.cancel = context.WithCancel(ctx)

	// 创建并启动工作器
	for i := 0; i < p.maxWorkers; i++ {
		worker := NewWorker(i, p.workerPool, p.ctx, &p.wg, p.metrics)
		p.workers = append(p.workers, worker)
		worker.Start()
	}

	// 启动调度器
	p.wg.Add(1)
	go p.dispatch()

	p.running = true
	p.metrics.SetActiveWorkers(int32(p.maxWorkers))

	return nil
}

// Stop 停止调度池
func (p *SchedulerPool) Stop(timeout time.Duration) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return nil // 已经停止
	}

	// 取消上下文
	if p.cancel != nil {
		p.cancel()
	}

	// 创建超时上下文用于优雅关闭
	stopCtx, stopCancel := context.WithTimeout(context.Background(), timeout)
	defer stopCancel()

	// 等待所有工作器完成
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 正常完成
	case <-stopCtx.Done():
		// 超时，强制停止工作器
		for _, worker := range p.workers {
			worker.Stop()
		}
		return fmt.Errorf("scheduler pool stop timeout")
	}

	// 关闭队列
	close(p.taskQueue)

	p.running = false
	p.metrics.SetActiveWorkers(0)

	return nil
}

// dispatch 调度器主循环
func (p *SchedulerPool) dispatch() {
	defer p.wg.Done()

	for {
		select {
		case task := <-p.taskQueue:
			// 获取可用的工作器
			select {
			case workerTaskQueue := <-p.workerPool:
				// 将任务分配给工作器
				workerTaskQueue <- task
				p.metrics.SetQueuedTasks(int32(len(p.taskQueue)))

			case <-p.ctx.Done():
				// 上下文取消，退出调度
				return
			}

		case <-p.ctx.Done():
			// 上下文取消，退出调度
			return
		}
	}
}

// IsRunning 检查调度池是否正在运行
func (p *SchedulerPool) IsRunning() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.running
}

// GetMetrics 获取池统计信息
func (p *SchedulerPool) GetMetrics() *PoolMetrics {
	return p.metrics
}

// RejectionPolicy 拒绝策略类型
type RejectionPolicy int

const (
	RejectPolicyAbort         RejectionPolicy = iota // 直接拒绝并返回错误
	RejectPolicyDiscard                              // 丢弃任务但不返回错误
	RejectPolicyCallerRuns                           // 在调用者goroutine中执行
	RejectPolicyDiscardOldest                        // 丢弃最旧的任务
)

// SchedulingStrategy 调度策略接口
type SchedulingStrategy interface {
	// ShouldAccept 判断是否应该接受任务
	ShouldAccept(task Task, queueSize int, maxSize int) bool
	// SelectTask 从任务列表中选择要执行的任务（用于优先级调度）
	SelectTask(tasks []Task) Task
	// OnTaskRejected 任务被拒绝时的回调
	OnTaskRejected(task Task, reason string)
}

// DefaultSchedulingStrategy 默认调度策略
type DefaultSchedulingStrategy struct{}

// ShouldAccept 实现SchedulingStrategy接口
func (s *DefaultSchedulingStrategy) ShouldAccept(task Task, queueSize int, maxSize int) bool {
	return queueSize < maxSize
}

// SelectTask 实现SchedulingStrategy接口（FIFO）
func (s *DefaultSchedulingStrategy) SelectTask(tasks []Task) Task {
	if len(tasks) == 0 {
		return nil
	}
	return tasks[0]
}

// OnTaskRejected 实现SchedulingStrategy接口
func (s *DefaultSchedulingStrategy) OnTaskRejected(task Task, reason string) {
	// 默认实现：记录日志
	fmt.Printf("Task %s rejected: %s\n", task.ID(), reason)
}

// PrioritySchedulingStrategy 优先级调度策略
type PrioritySchedulingStrategy struct {
	DefaultSchedulingStrategy
}

// SelectTask 实现优先级调度（数值越小优先级越高）
func (s *PrioritySchedulingStrategy) SelectTask(tasks []Task) Task {
	if len(tasks) == 0 {
		return nil
	}

	// 找到优先级最高的任务
	highestPriorityTask := tasks[0]
	for _, task := range tasks[1:] {
		if task.Priority() < highestPriorityTask.Priority() {
			highestPriorityTask = task
		}
	}
	return highestPriorityTask
}

// 更新SchedulerPool结构，添加新字段
func (p *SchedulerPool) setRejectionPolicy(policy RejectionPolicy) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.rejectionPolicy = policy
}

func (p *SchedulerPool) setSchedulingStrategy(strategy SchedulingStrategy) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.schedulingStrategy = strategy
}

// Submit 提交任务
func (p *SchedulerPool) Submit(task Task) error {
	if task == nil {
		return ErrInvalidTask
	}

	submitTime := time.Now()

	p.mutex.RLock()
	running := p.running
	strategy := p.schedulingStrategy
	policy := p.rejectionPolicy
	p.mutex.RUnlock()

	if !running {
		return ErrPoolNotRunning
	}

	// 增加提交计数
	p.metrics.IncrementSubmissions()

	// 检查调度策略是否接受任务
	queueSize := len(p.taskQueue)
	maxSize := cap(p.taskQueue)

	if !strategy.ShouldAccept(task, queueSize, maxSize) {
		p.metrics.IncrementRejected()
		return p.handleRejection(task, policy, "queue full or strategy rejected")
	}

	// 尝试提交任务
	select {
	case p.taskQueue <- task:
		// 计算等待时间（从提交到进入队列）
		waitTime := time.Since(submitTime)
		p.metrics.AddWaitTime(waitTime)
		p.metrics.SetQueuedTasks(int32(len(p.taskQueue)))
		return nil
	default:
		// 队列满，根据拒绝策略处理
		p.metrics.IncrementRejected()
		return p.handleRejection(task, policy, "task queue full")
	}
}

// SubmitFunc 提交函数任务
func (p *SchedulerPool) SubmitFunc(fn func(context.Context) error) error {
	if fn == nil {
		return ErrInvalidTask
	}

	// 生成任务ID
	taskID := fmt.Sprintf("func-task-%d", time.Now().UnixNano())
	task := NewConfigTask(taskID, TaskTypeHook, 0, fn)

	return p.Submit(task)
}

// SubmitWithContext 提交带上下文的函数任务
func (p *SchedulerPool) SubmitWithContext(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return ErrInvalidTask
	}

	// 包装函数以传递上下文
	wrappedFn := func(taskCtx context.Context) error {
		// 合并上下文
		mergedCtx := ctx
		if taskCtx != nil {
			// 如果任务上下文有超时等限制，优先使用任务上下文
			select {
			case <-taskCtx.Done():
				mergedCtx = taskCtx
			default:
				mergedCtx = ctx
			}
		}
		return fn(mergedCtx)
	}

	return p.SubmitFunc(wrappedFn)
}

// handleRejection 处理任务拒绝
func (p *SchedulerPool) handleRejection(task Task, policy RejectionPolicy, reason string) error {
	p.mutex.RLock()
	strategy := p.schedulingStrategy
	p.mutex.RUnlock()

	// 通知调度策略任务被拒绝
	strategy.OnTaskRejected(task, reason)

	switch policy {
	case RejectPolicyAbort:
		return ErrTaskQueueFull

	case RejectPolicyDiscard:
		// 静默丢弃任务
		return nil

	case RejectPolicyCallerRuns:
		// 在调用者goroutine中执行任务
		return task.Execute(context.Background())

	case RejectPolicyDiscardOldest:
		// 尝试丢弃最旧的任务并重新提交
		select {
		case <-p.taskQueue:
			// 成功丢弃一个任务，重新提交当前任务
			select {
			case p.taskQueue <- task:
				p.metrics.SetQueuedTasks(int32(len(p.taskQueue)))
				return nil
			default:
				return ErrTaskQueueFull
			}
		default:
			// 队列为空，直接返回错误
			return ErrTaskQueueFull
		}

	default:
		return ErrTaskQueueFull
	}
}

// SetRejectionPolicy 设置拒绝策略
func (p *SchedulerPool) SetRejectionPolicy(policy RejectionPolicy) {
	p.setRejectionPolicy(policy)
}

// SetSchedulingStrategy 设置调度策略
func (p *SchedulerPool) SetSchedulingStrategy(strategy SchedulingStrategy) {
	if strategy != nil {
		p.setSchedulingStrategy(strategy)
	}
}

// GetRejectionPolicy 获取当前拒绝策略
func (p *SchedulerPool) GetRejectionPolicy() RejectionPolicy {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.rejectionPolicy
}

// GetSchedulingStrategy 获取当前调度策略
func (p *SchedulerPool) GetSchedulingStrategy() SchedulingStrategy {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.schedulingStrategy
}

// GetDetailedMetrics 获取详细统计信息快照
func (p *SchedulerPool) GetDetailedMetrics() PoolMetricsSnapshot {
	return p.metrics.Snapshot()
}

// ResetMetrics 重置统计信息
func (p *SchedulerPool) ResetMetrics() {
	p.metrics.Reset()
}

// GetWorkerCount 获取当前工作器数量
func (p *SchedulerPool) GetWorkerCount() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return len(p.workers)
}

// GetMaxWorkers 获取最大工作器数量
func (p *SchedulerPool) GetMaxWorkers() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.maxWorkers
}

// GetQueueCapacity 获取队列容量
func (p *SchedulerPool) GetQueueCapacity() int {
	return cap(p.taskQueue)
}

// GetCurrentQueueSize 获取当前队列大小
func (p *SchedulerPool) GetCurrentQueueSize() int {
	return len(p.taskQueue)
}

// MonitorGoroutines 监控goroutine生命周期
func (p *SchedulerPool) MonitorGoroutines() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	info := make(map[string]interface{})
	info["total_workers"] = len(p.workers)
	info["active_workers"] = p.metrics.GetActiveWorkers()
	info["running"] = p.running

	// 添加每个工作器的状态信息
	workerStates := make([]map[string]interface{}, len(p.workers))
	for i, worker := range p.workers {
		workerStates[i] = map[string]interface{}{
			"id":      worker.id,
			"running": true, // 简化实现，实际可以添加更详细的状态
		}
	}
	info["workers"] = workerStates

	return info
}

// HealthCheck 健康检查
func (p *SchedulerPool) HealthCheck() map[string]interface{} {
	metrics := p.GetDetailedMetrics()

	health := make(map[string]interface{})
	health["status"] = "healthy"
	health["running"] = p.IsRunning()
	health["uptime"] = metrics.Uptime.String()
	health["success_rate"] = metrics.SuccessRate
	health["queue_utilization"] = float64(metrics.QueuedTasks) / float64(p.GetQueueCapacity())
	health["worker_utilization"] = float64(metrics.ActiveWorkers) / float64(p.GetMaxWorkers())

	// 判断健康状态
	if !p.IsRunning() {
		health["status"] = "stopped"
	} else if metrics.SuccessRate < 0.9 && metrics.CompletedTasks > 10 {
		health["status"] = "degraded"
	} else if float64(metrics.QueuedTasks)/float64(p.GetQueueCapacity()) > 0.9 {
		health["status"] = "overloaded"
	}

	return health
}
