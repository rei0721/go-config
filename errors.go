package config

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

// Logger 日志记录器接口
type Logger interface {
	Debug(ctx context.Context, msg string, fields map[string]interface{})
	Info(ctx context.Context, msg string, fields map[string]interface{})
	Warn(ctx context.Context, msg string, fields map[string]interface{})
	Error(ctx context.Context, msg string, fields map[string]interface{})
}

// s2error formats according to a format specifier and returns the resulting string.
func s2error(s string, a ...any) error {
	//s2 := Sprintf("%s", s)
	return errors.New(fmt.Sprintf(s, a...))
}

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	HandleError(ctx context.Context, err error) error
	CanRecover(err error) bool
	GetRecoveryStrategy(err error) RecoveryStrategy
}

// RecoveryStrategy 恢复策略接口
type RecoveryStrategy interface {
	Recover(ctx context.Context, err error) error
	GetRetryCount() int
	GetRetryDelay() time.Duration
}

// ErrorContext 错误上下文
type ErrorContext struct {
	Operation   string                 // 操作名称
	Component   string                 // 组件名称
	Timestamp   time.Time              // 错误发生时间
	StackTrace  string                 // 堆栈跟踪
	Metadata    map[string]interface{} // 元数据
	Recoverable bool                   // 是否可恢复
	Error       error                  // 原始错误
}

// NewErrorContext 创建新的错误上下文
func NewErrorContext(operation, component string, err error) *ErrorContext {
	return &ErrorContext{
		Operation:   operation,
		Component:   component,
		Timestamp:   time.Now(),
		StackTrace:  getStackTrace(),
		Metadata:    make(map[string]interface{}),
		Recoverable: false,
		Error:       err,
	}
}

// WithMetadata 添加元数据
func (ec *ErrorContext) WithMetadata(key string, value interface{}) *ErrorContext {
	ec.Metadata[key] = value
	return ec
}

// WithRecoverable 设置是否可恢复
func (ec *ErrorContext) WithRecoverable(recoverable bool) *ErrorContext {
	ec.Recoverable = recoverable
	return ec
}

// String 返回错误上下文的字符串表示
func (ec *ErrorContext) String() string {
	return fmt.Sprintf("[%s] %s.%s: %v (recoverable: %t)",
		ec.Timestamp.Format(time.RFC3339),
		ec.Component,
		ec.Operation,
		ec.Error,
		ec.Recoverable)
}

// getStackTrace 获取堆栈跟踪
func getStackTrace() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

// ErrorType 错误类型枚举
type ErrorType int

const (
	ErrorTypeConfig      ErrorType = iota // 配置错误
	ErrorTypeConcurrency                  // 并发错误
	ErrorTypeResource                     // 资源错误
	ErrorTypeNetwork                      // 网络错误
	ErrorTypeBusiness                     // 业务错误
	ErrorTypeSystem                       // 系统错误
)

// String 返回错误类型的字符串表示
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeConfig:
		return "Config"
	case ErrorTypeConcurrency:
		return "Concurrency"
	case ErrorTypeResource:
		return "Resource"
	case ErrorTypeNetwork:
		return "Network"
	case ErrorTypeBusiness:
		return "Business"
	case ErrorTypeSystem:
		return "System"
	default:
		return "Unknown"
	}
}

// StructuredError 结构化错误
type StructuredError struct {
	Type      ErrorType              // 错误类型
	Code      string                 // 错误代码
	Message   string                 // 错误消息
	Cause     error                  // 原因错误
	Context   *ErrorContext          // 错误上下文
	Metadata  map[string]interface{} // 附加元数据
	Timestamp time.Time              // 错误时间
}

// NewStructuredError 创建新的结构化错误
func NewStructuredError(errorType ErrorType, code, message string) *StructuredError {
	return &StructuredError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// WithCause 设置原因错误
func (se *StructuredError) WithCause(cause error) *StructuredError {
	se.Cause = cause
	return se
}

// WithContext 设置错误上下文
func (se *StructuredError) WithContext(ctx *ErrorContext) *StructuredError {
	se.Context = ctx
	return se
}

// WithMetadata 添加元数据
func (se *StructuredError) WithMetadata(key string, value interface{}) *StructuredError {
	se.Metadata[key] = value
	return se
}

// Error 实现error接口
func (se *StructuredError) Error() string {
	if se.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", se.Type, se.Code, se.Message, se.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", se.Type, se.Code, se.Message)
}

// Unwrap 实现errors.Unwrap接口
func (se *StructuredError) Unwrap() error {
	return se.Cause
}

// Is 实现errors.Is接口
func (se *StructuredError) Is(target error) bool {
	if target == nil {
		return false
	}

	if structuredErr, ok := target.(*StructuredError); ok {
		return se.Type == structuredErr.Type && se.Code == structuredErr.Code
	}

	return errors.Is(se.Cause, target)
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct {
	logger Logger // 日志记录器
}

// NewDefaultErrorHandler 创建默认错误处理器
func NewDefaultErrorHandler(logger Logger) *DefaultErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// HandleError 处理错误
func (h *DefaultErrorHandler) HandleError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// 记录错误日志
	if h.logger != nil {
		h.logger.Error(ctx, "Error occurred", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 如果是结构化错误，记录详细信息
	if structuredErr, ok := err.(*StructuredError); ok {
		if h.logger != nil {
			h.logger.Error(ctx, "Structured error details", map[string]interface{}{
				"type":      structuredErr.Type.String(),
				"code":      structuredErr.Code,
				"message":   structuredErr.Message,
				"metadata":  structuredErr.Metadata,
				"timestamp": structuredErr.Timestamp,
			})
		}
	}

	return err
}

// CanRecover 判断错误是否可恢复
func (h *DefaultErrorHandler) CanRecover(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否是结构化错误
	if structuredErr, ok := err.(*StructuredError); ok {
		if structuredErr.Context != nil {
			return structuredErr.Context.Recoverable
		}

		// 根据错误类型判断是否可恢复
		switch structuredErr.Type {
		case ErrorTypeNetwork, ErrorTypeResource:
			return true
		case ErrorTypeConfig, ErrorTypeBusiness:
			return false
		default:
			return false
		}
	}

	// 检查预定义错误
	switch err {
	case ErrTaskQueueFull, ErrWorkerTimeout, ErrWatcherStartFailed:
		return true
	case ErrInvalidHandler, ErrInvalidTask, ErrConfigNil:
		return false
	default:
		return false
	}
}

// GetRecoveryStrategy 获取恢复策略
func (h *DefaultErrorHandler) GetRecoveryStrategy(err error) RecoveryStrategy {
	if !h.CanRecover(err) {
		return &NoRecoveryStrategy{}
	}

	// 根据错误类型返回不同的恢复策略
	if structuredErr, ok := err.(*StructuredError); ok {
		switch structuredErr.Type {
		case ErrorTypeNetwork:
			return &ExponentialBackoffStrategy{
				maxRetries: 3,
				baseDelay:  time.Second,
			}
		case ErrorTypeResource:
			return &LinearBackoffStrategy{
				maxRetries: 5,
				delay:      500 * time.Millisecond,
			}
		default:
			return &SimpleRetryStrategy{
				maxRetries: 1,
				delay:      100 * time.Millisecond,
			}
		}
	}

	return &SimpleRetryStrategy{
		maxRetries: 1,
		delay:      100 * time.Millisecond,
	}
}

// NoRecoveryStrategy 无恢复策略
type NoRecoveryStrategy struct{}

func (s *NoRecoveryStrategy) Recover(ctx context.Context, err error) error {
	return err // 不进行恢复
}

func (s *NoRecoveryStrategy) GetRetryCount() int {
	return 0
}

func (s *NoRecoveryStrategy) GetRetryDelay() time.Duration {
	return 0
}

// SimpleRetryStrategy 简单重试策略
type SimpleRetryStrategy struct {
	maxRetries int
	delay      time.Duration
}

func (s *SimpleRetryStrategy) Recover(ctx context.Context, err error) error {
	// 简单延迟后返回，由调用方决定是否重试
	select {
	case <-time.After(s.delay):
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *SimpleRetryStrategy) GetRetryCount() int {
	return s.maxRetries
}

func (s *SimpleRetryStrategy) GetRetryDelay() time.Duration {
	return s.delay
}

// ExponentialBackoffStrategy 指数退避策略
type ExponentialBackoffStrategy struct {
	maxRetries int
	baseDelay  time.Duration
	attempt    int
}

func (s *ExponentialBackoffStrategy) Recover(ctx context.Context, err error) error {
	if s.attempt >= s.maxRetries {
		return err
	}

	// 计算指数退避延迟
	delay := s.baseDelay * time.Duration(1<<s.attempt)
	s.attempt++

	select {
	case <-time.After(delay):
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *ExponentialBackoffStrategy) GetRetryCount() int {
	return s.maxRetries
}

func (s *ExponentialBackoffStrategy) GetRetryDelay() time.Duration {
	return s.baseDelay * time.Duration(1<<s.attempt)
}

// LinearBackoffStrategy 线性退避策略
type LinearBackoffStrategy struct {
	maxRetries int
	delay      time.Duration
	attempt    int
}

func (s *LinearBackoffStrategy) Recover(ctx context.Context, err error) error {
	if s.attempt >= s.maxRetries {
		return err
	}

	// 线性增加延迟
	delay := s.delay * time.Duration(s.attempt+1)
	s.attempt++

	select {
	case <-time.After(delay):
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *LinearBackoffStrategy) GetRetryCount() int {
	return s.maxRetries
}

func (s *LinearBackoffStrategy) GetRetryDelay() time.Duration {
	return s.delay * time.Duration(s.attempt+1)
}

// Hook系统相关错误
var (
	ErrInvalidHandler           = NewStructuredError(ErrorTypeBusiness, "HOOK_001", "invalid handler: handler cannot be nil")
	ErrHandlerAlreadyRegistered = NewStructuredError(ErrorTypeBusiness, "HOOK_002", "handler already registered for this hook type")
	ErrHandlerNotFound          = NewStructuredError(ErrorTypeBusiness, "HOOK_003", "handler not found")
	ErrHookExecutionFailed      = NewStructuredError(ErrorTypeBusiness, "HOOK_004", "hook execution failed")
	ErrHookTimeout              = NewStructuredError(ErrorTypeSystem, "HOOK_005", "hook execution timeout")
)

// 调度池相关错误
var (
	ErrPoolNotRunning     = NewStructuredError(ErrorTypeSystem, "POOL_001", "scheduler pool is not running")
	ErrPoolAlreadyRunning = NewStructuredError(ErrorTypeSystem, "POOL_002", "scheduler pool is already running")
	ErrTaskQueueFull      = NewStructuredError(ErrorTypeResource, "POOL_003", "task queue is full")
	ErrInvalidTask        = NewStructuredError(ErrorTypeBusiness, "POOL_004", "invalid task: task cannot be nil")
	ErrWorkerTimeout      = NewStructuredError(ErrorTypeSystem, "POOL_005", "worker execution timeout")
	ErrPoolShutdown       = NewStructuredError(ErrorTypeSystem, "POOL_006", "scheduler pool is shutting down")
)

// 文件监听器相关错误
var (
	ErrWatcherNotRunning     = NewStructuredError(ErrorTypeSystem, "WATCHER_001", "file watcher is not running")
	ErrWatcherAlreadyRunning = NewStructuredError(ErrorTypeSystem, "WATCHER_002", "file watcher is already running")
	ErrInvalidFilepath       = NewStructuredError(ErrorTypeBusiness, "WATCHER_003", "invalid file path")
	ErrWatcherStartFailed    = NewStructuredError(ErrorTypeNetwork, "WATCHER_004", "failed to start file watcher")
	ErrWatcherStopTimeout    = NewStructuredError(ErrorTypeSystem, "WATCHER_005", "file watcher stop timeout")
	ErrInvalidFileHandler    = NewStructuredError(ErrorTypeBusiness, "WATCHER_006", "invalid file change handler")
)

// 配置存储相关错误
var (
	ErrConfigNil            = NewStructuredError(ErrorTypeBusiness, "CONFIG_001", "config is nil")
	ErrInvalidUpdateFunc    = NewStructuredError(ErrorTypeBusiness, "CONFIG_002", "update function cannot be nil")
	ErrSnapshotNotFound     = NewStructuredError(ErrorTypeBusiness, "CONFIG_003", "snapshot not found")
	ErrInvalidSnapshot      = NewStructuredError(ErrorTypeBusiness, "CONFIG_004", "invalid snapshot")
	ErrConfigUpdateFailed   = NewStructuredError(ErrorTypeSystem, "CONFIG_005", "config update failed")
	ErrSnapshotCreateFailed = NewStructuredError(ErrorTypeSystem, "CONFIG_006", "failed to create snapshot")
)
