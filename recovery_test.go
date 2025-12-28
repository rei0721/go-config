package config

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRecoveryManager_BackupAndRestore(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "config_backup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试组件
	logger := NewNoOpLogger()
	errorHandler := NewDefaultErrorHandler(logger)
	recoveryManager := NewRecoveryManager(errorHandler, logger, tempDir, 3)

	ctx := context.Background()

	// 测试配置数据
	testConfig := map[string]interface{}{
		"database": map[string]interface{}{
			"host": "localhost",
			"port": 5432,
		},
		"cache": map[string]interface{}{
			"enabled": true,
			"ttl":     300,
		},
	}

	// 测试备份
	version := int64(1)
	err = recoveryManager.BackupConfig(ctx, testConfig, version)
	if err != nil {
		t.Fatalf("Failed to backup config: %v", err)
	}

	// 验证备份文件存在
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read backup dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 backup file, got %d", len(files))
	}

	// 测试恢复
	restoredConfig, err := recoveryManager.RestoreConfig(ctx, version)
	if err != nil {
		t.Fatalf("Failed to restore config: %v", err)
	}

	// 验证恢复的配置
	if restoredConfig == nil {
		t.Fatal("Restored config is nil")
	}

	// 测试获取最新备份
	latestConfig, err := recoveryManager.GetLatestBackup(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest backup: %v", err)
	}
	if latestConfig == nil {
		t.Fatal("Latest config is nil")
	}
}

func TestDefaultLogger_LogLevels(t *testing.T) {
	// 创建临时文件用于日志输出
	tempFile, err := os.CreateTemp("", "log_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 创建日志记录器
	logger := NewDefaultLogger(LogLevelInfo, tempFile, "test")

	ctx := context.Background()

	// 测试不同级别的日志
	logger.Debug(ctx, "Debug message", map[string]interface{}{"key": "debug"})
	logger.Info(ctx, "Info message", map[string]interface{}{"key": "info"})
	logger.Warn(ctx, "Warn message", map[string]interface{}{"key": "warn"})
	logger.Error(ctx, "Error message", map[string]interface{}{"key": "error"})

	// 读取日志文件内容
	tempFile.Seek(0, 0)
	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Debug 消息不应该出现（级别太低）
	if contains(logContent, "Debug message") {
		t.Error("Debug message should not appear with Info level")
	}

	// Info, Warn, Error 消息应该出现
	if !contains(logContent, "Info message") {
		t.Error("Info message should appear")
	}
	if !contains(logContent, "Warn message") {
		t.Error("Warn message should appear")
	}
	if !contains(logContent, "Error message") {
		t.Error("Error message should appear")
	}
}

func TestStructuredError(t *testing.T) {
	// 创建结构化错误
	err := NewStructuredError(ErrorTypeConfig, "TEST_001", "test error message")

	if err.Type != ErrorTypeConfig {
		t.Errorf("Expected error type %v, got %v", ErrorTypeConfig, err.Type)
	}

	if err.Code != "TEST_001" {
		t.Errorf("Expected error code TEST_001, got %s", err.Code)
	}

	if err.Message != "test error message" {
		t.Errorf("Expected message 'test error message', got %s", err.Message)
	}

	// 测试链式调用
	causeErr := NewStructuredError(ErrorTypeSystem, "SYS_001", "system error")
	err = err.WithCause(causeErr).WithMetadata("key", "value")

	if err.Cause != causeErr {
		t.Error("Cause error not set correctly")
	}

	if err.Metadata["key"] != "value" {
		t.Error("Metadata not set correctly")
	}
}

func TestErrorHandler_CanRecover(t *testing.T) {
	logger := NewNoOpLogger()
	handler := NewDefaultErrorHandler(logger)

	// 测试可恢复错误
	recoverableErr := NewStructuredError(ErrorTypeNetwork, "NET_001", "network error")
	if !handler.CanRecover(recoverableErr) {
		t.Error("Network error should be recoverable")
	}

	// 测试不可恢复错误
	nonRecoverableErr := NewStructuredError(ErrorTypeConfig, "CFG_001", "config error")
	if handler.CanRecover(nonRecoverableErr) {
		t.Error("Config error should not be recoverable")
	}

	// 测试预定义错误
	if !handler.CanRecover(ErrTaskQueueFull) {
		t.Error("Task queue full error should be recoverable")
	}

	if handler.CanRecover(ErrInvalidHandler) {
		t.Error("Invalid handler error should not be recoverable")
	}
}

func TestRecoveryStrategies(t *testing.T) {
	ctx := context.Background()

	// 测试简单重试策略
	simpleStrategy := &SimpleRetryStrategy{
		maxRetries: 2,
		delay:      10 * time.Millisecond,
	}

	if simpleStrategy.GetRetryCount() != 2 {
		t.Errorf("Expected retry count 2, got %d", simpleStrategy.GetRetryCount())
	}

	if simpleStrategy.GetRetryDelay() != 10*time.Millisecond {
		t.Errorf("Expected delay 10ms, got %v", simpleStrategy.GetRetryDelay())
	}

	// 测试指数退避策略
	expStrategy := &ExponentialBackoffStrategy{
		maxRetries: 3,
		baseDelay:  10 * time.Millisecond,
	}

	if expStrategy.GetRetryCount() != 3 {
		t.Errorf("Expected retry count 3, got %d", expStrategy.GetRetryCount())
	}

	// 测试线性退避策略
	linearStrategy := &LinearBackoffStrategy{
		maxRetries: 2,
		delay:      10 * time.Millisecond,
	}

	if linearStrategy.GetRetryCount() != 2 {
		t.Errorf("Expected retry count 2, got %d", linearStrategy.GetRetryCount())
	}

	// 测试无恢复策略
	noStrategy := &NoRecoveryStrategy{}

	if noStrategy.GetRetryCount() != 0 {
		t.Errorf("Expected retry count 0, got %d", noStrategy.GetRetryCount())
	}

	testErr := NewStructuredError(ErrorTypeSystem, "TEST", "test error")
	if err := noStrategy.Recover(ctx, testErr); err != testErr {
		t.Error("NoRecoveryStrategy should return the original error")
	}
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
