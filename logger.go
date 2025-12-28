package config

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry 日志条目
type LogEntry struct {
	Level     LogLevel               `json:"level"`
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
	Component string                 `json:"component"`
}

// DefaultLogger 默认日志记录器实现
type DefaultLogger struct {
	level     LogLevel
	output    io.Writer
	logger    *log.Logger
	component string
	mutex     sync.RWMutex
}

// NewDefaultLogger 创建默认日志记录器
func NewDefaultLogger(level LogLevel, output io.Writer, component string) *DefaultLogger {
	if output == nil {
		output = os.Stdout
	}

	return &DefaultLogger{
		level:     level,
		output:    output,
		logger:    log.New(output, "", 0),
		component: component,
	}
}

// SetLevel 设置日志级别
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.level = level
}

// GetLevel 获取当前日志级别
func (l *DefaultLogger) GetLevel() LogLevel {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.level
}

// Debug 记录调试日志
func (l *DefaultLogger) Debug(ctx context.Context, msg string, fields map[string]interface{}) {
	l.log(LogLevelDebug, msg, fields)
}

// Info 记录信息日志
func (l *DefaultLogger) Info(ctx context.Context, msg string, fields map[string]interface{}) {
	l.log(LogLevelInfo, msg, fields)
}

// Warn 记录警告日志
func (l *DefaultLogger) Warn(ctx context.Context, msg string, fields map[string]interface{}) {
	l.log(LogLevelWarn, msg, fields)
}

// Error 记录错误日志
func (l *DefaultLogger) Error(ctx context.Context, msg string, fields map[string]interface{}) {
	l.log(LogLevelError, msg, fields)
}

// log 内部日志记录方法
func (l *DefaultLogger) log(level LogLevel, msg string, fields map[string]interface{}) {
	l.mutex.RLock()
	currentLevel := l.level
	l.mutex.RUnlock()

	// 检查日志级别
	if level < currentLevel {
		return
	}

	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Message:   msg,
		Fields:    fields,
		Component: l.component,
	}

	// 格式化日志输出
	logLine := l.formatLogEntry(entry)

	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Print(logLine)
}

// formatLogEntry 格式化日志条目
func (l *DefaultLogger) formatLogEntry(entry LogEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05.000")

	fieldsStr := ""
	if len(entry.Fields) > 0 {
		fieldsStr = " "
		for k, v := range entry.Fields {
			fieldsStr += fmt.Sprintf("%s=%v ", k, v)
		}
	}

	return fmt.Sprintf("[%s] %s [%s] %s%s",
		timestamp,
		entry.Level.String(),
		entry.Component,
		entry.Message,
		fieldsStr)
}

// NoOpLogger 空操作日志记录器（用于禁用日志）
type NoOpLogger struct{}

// NewNoOpLogger 创建空操作日志记录器
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

func (l *NoOpLogger) Debug(ctx context.Context, msg string, fields map[string]interface{}) {}
func (l *NoOpLogger) Info(ctx context.Context, msg string, fields map[string]interface{})  {}
func (l *NoOpLogger) Warn(ctx context.Context, msg string, fields map[string]interface{})  {}
func (l *NoOpLogger) Error(ctx context.Context, msg string, fields map[string]interface{}) {}
