package config

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HookExecutor 钩子执行器
type HookExecutor struct {
	registry      *HookRegistry          // 钩子注册表
	schedulerPool SchedulerPoolInterface // 调度池接口
	timeout       time.Duration          // 执行超时时间
	mutex         sync.RWMutex           // 保护并发访问
	metrics       *ExecutorMetrics       // 执行统计
}

// ExecutorMetrics 执行器统计信息
type ExecutorMetrics struct {
	TotalExecutions int64         // 总执行次数
	SuccessfulExecs int64         // 成功执行次数
	FailedExecs     int64         // 失败执行次数
	AverageExecTime time.Duration // 平均执行时间
	LastExecution   time.Time     // 最后执行时间
	mutex           sync.RWMutex  // 保护统计数据
}

// SchedulerPoolInterface 调度池接口
type SchedulerPoolInterface interface {
	Submit(task Task) error
	SubmitFunc(fn func(context.Context) error) error
	SubmitWithContext(ctx context.Context, fn func(context.Context) error) error
}

// NewHookExecutor 创建新的钩子执行器
func NewHookExecutor(registry *HookRegistry, schedulerPool SchedulerPoolInterface, timeout time.Duration) *HookExecutor {
	if timeout <= 0 {
		timeout = 30 * time.Second // 默认30秒超时
	}

	return &HookExecutor{
		registry:      registry,
		schedulerPool: schedulerPool,
		timeout:       timeout,
		metrics:       &ExecutorMetrics{},
	}
}

// Execute 执行钩子
func (e *HookExecutor) Execute(ctx context.Context, hookCtx *HookContext) error {
	if hookCtx == nil {
		return fmt.Errorf("hook context cannot be nil")
	}

	// 设置时间戳
	if hookCtx.Timestamp.IsZero() {
		hookCtx.Timestamp = time.Now()
	}

	// 获取处理器
	handlers := e.registry.GetHandlers(hookCtx.Type)
	if len(handlers) == 0 {
		return nil // 没有处理器，直接返回
	}

	startTime := time.Now()
	defer func() {
		e.updateMetrics(startTime, nil)
	}()

	// 分离同步和异步处理器
	var syncHandlers, asyncHandlers []HookHandler
	for _, handler := range handlers {
		if handler.IsAsync() {
			asyncHandlers = append(asyncHandlers, handler)
		} else {
			syncHandlers = append(syncHandlers, handler)
		}
	}

	var allErrors []error

	// 执行同步处理器
	if len(syncHandlers) > 0 {
		if err := e.executeSyncHandlers(ctx, hookCtx, syncHandlers); err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// 执行异步处理器
	if len(asyncHandlers) > 0 {
		if err := e.executeAsyncHandlers(ctx, hookCtx, asyncHandlers); err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// 合并错误
	if len(allErrors) > 0 {
		e.updateMetrics(startTime, allErrors[0])
		return fmt.Errorf("hook execution failed: %v", allErrors)
	}

	return nil
}

// executeSyncHandlers 执行同步处理器
func (e *HookExecutor) executeSyncHandlers(ctx context.Context, hookCtx *HookContext, handlers []HookHandler) error {
	var errors []error

	for _, handler := range handlers {
		if err := e.executeHandlerWithTimeout(ctx, handler, hookCtx); err != nil {
			// 错误隔离：一个处理器失败不影响其他处理器
			errors = append(errors, fmt.Errorf("handler execution failed: %w", err))
			continue
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("sync handlers failed: %v", errors)
	}

	return nil
}

// executeAsyncHandlers 执行异步处理器
func (e *HookExecutor) executeAsyncHandlers(ctx context.Context, hookCtx *HookContext, handlers []HookHandler) error {
	if e.schedulerPool == nil {
		// 如果没有调度池，降级为同步执行
		return e.executeSyncHandlers(ctx, hookCtx, handlers)
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		h := handler // 避免闭包问题

		err := e.schedulerPool.SubmitWithContext(ctx, func(taskCtx context.Context) error {
			defer wg.Done()
			if err := e.executeHandlerWithTimeout(taskCtx, h, hookCtx); err != nil {
				errorChan <- fmt.Errorf("async handler execution failed: %w", err)
			}
			return nil
		})

		if err != nil {
			wg.Done()
			errorChan <- fmt.Errorf("failed to submit async handler: %w", err)
		}
	}

	// 等待所有异步处理器完成
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// 收集错误
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("async handlers failed: %v", errors)
	}

	return nil
}

// executeHandlerWithTimeout 带超时的处理器执行
func (e *HookExecutor) executeHandlerWithTimeout(ctx context.Context, handler HookHandler, hookCtx *HookContext) error {
	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// 使用通道来处理超时
	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("handler panicked: %v", r)
			}
		}()

		done <- handler.Handle(hookCtx)
	}()

	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		return ErrHookTimeout
	}
}

// updateMetrics 更新执行统计
func (e *HookExecutor) updateMetrics(startTime time.Time, err error) {
	e.metrics.mutex.Lock()
	defer e.metrics.mutex.Unlock()

	e.metrics.TotalExecutions++
	e.metrics.LastExecution = time.Now()

	execTime := time.Since(startTime)

	// 计算平均执行时间
	if e.metrics.TotalExecutions == 1 {
		e.metrics.AverageExecTime = execTime
	} else {
		// 使用移动平均
		e.metrics.AverageExecTime = (e.metrics.AverageExecTime + execTime) / 2
	}

	if err != nil {
		e.metrics.FailedExecs++
	} else {
		e.metrics.SuccessfulExecs++
	}
}

// GetMetrics 获取执行统计
func (e *HookExecutor) GetMetrics() ExecutorMetrics {
	e.metrics.mutex.RLock()
	defer e.metrics.mutex.RUnlock()

	return ExecutorMetrics{
		TotalExecutions: e.metrics.TotalExecutions,
		SuccessfulExecs: e.metrics.SuccessfulExecs,
		FailedExecs:     e.metrics.FailedExecs,
		AverageExecTime: e.metrics.AverageExecTime,
		LastExecution:   e.metrics.LastExecution,
	}
}

// SetTimeout 设置执行超时时间
func (e *HookExecutor) SetTimeout(timeout time.Duration) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.timeout = timeout
}

// GetTimeout 获取当前超时时间
func (e *HookExecutor) GetTimeout() time.Duration {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.timeout
}
