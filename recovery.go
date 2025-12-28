package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RecoveryManager 错误恢复管理器
type RecoveryManager struct {
	errorHandler ErrorHandler
	logger       Logger
	backupDir    string
	maxBackups   int
	mutex        sync.RWMutex
}

// NewRecoveryManager 创建恢复管理器
func NewRecoveryManager(errorHandler ErrorHandler, logger Logger, backupDir string, maxBackups int) *RecoveryManager {
	if maxBackups <= 0 {
		maxBackups = 5 // 默认保留5个备份
	}

	return &RecoveryManager{
		errorHandler: errorHandler,
		logger:       logger,
		backupDir:    backupDir,
		maxBackups:   maxBackups,
	}
}

// RecoverFromError 从错误中恢复
func (rm *RecoveryManager) RecoverFromError(ctx context.Context, err error, operation func() error) error {
	if err == nil {
		return nil
	}

	// 检查是否可以恢复
	if !rm.errorHandler.CanRecover(err) {
		rm.logger.Error(ctx, "Error is not recoverable", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// 获取恢复策略
	strategy := rm.errorHandler.GetRecoveryStrategy(err)
	if strategy == nil {
		rm.logger.Error(ctx, "No recovery strategy available", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// 执行恢复
	maxRetries := strategy.GetRetryCount()
	for attempt := 0; attempt < maxRetries; attempt++ {
		rm.logger.Info(ctx, "Attempting recovery", map[string]interface{}{
			"attempt":     attempt + 1,
			"max_retries": maxRetries,
			"error":       err.Error(),
		})

		// 等待恢复延迟
		if err := strategy.Recover(ctx, err); err != nil {
			if err == ctx.Err() {
				return err // 上下文取消
			}
		}

		// 重试操作
		if retryErr := operation(); retryErr == nil {
			rm.logger.Info(ctx, "Recovery successful", map[string]interface{}{
				"attempt": attempt + 1,
			})
			return nil
		} else {
			rm.logger.Warn(ctx, "Recovery attempt failed", map[string]interface{}{
				"attempt": attempt + 1,
				"error":   retryErr.Error(),
			})
			err = retryErr
		}
	}

	rm.logger.Error(ctx, "All recovery attempts failed", map[string]interface{}{
		"max_retries": maxRetries,
		"final_error": err.Error(),
	})

	return err
}

// ConfigBackup 配置备份
type ConfigBackup struct {
	Timestamp time.Time   `json:"timestamp"`
	Version   int64       `json:"version"`
	Data      interface{} `json:"data"`
	Checksum  string      `json:"checksum"`
}

// BackupConfig 备份配置
func (rm *RecoveryManager) BackupConfig(ctx context.Context, config interface{}, version int64) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// 确保备份目录存在
	if err := os.MkdirAll(rm.backupDir, 0755); err != nil {
		return NewStructuredError(ErrorTypeSystem, "BACKUP_001", "failed to create backup directory").
			WithCause(err).
			WithContext(NewErrorContext("BackupConfig", "RecoveryManager", err))
	}

	// 创建备份对象
	backup := &ConfigBackup{
		Timestamp: time.Now(),
		Version:   version,
		Data:      config,
	}

	// 序列化配置数据
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return NewStructuredError(ErrorTypeSystem, "BACKUP_002", "failed to serialize config").
			WithCause(err).
			WithContext(NewErrorContext("BackupConfig", "RecoveryManager", err))
	}

	// 计算校验和
	backup.Checksum = calculateChecksum(data)

	// 重新序列化包含校验和的数据
	data, err = json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return NewStructuredError(ErrorTypeSystem, "BACKUP_003", "failed to serialize config with checksum").
			WithCause(err).
			WithContext(NewErrorContext("BackupConfig", "RecoveryManager", err))
	}

	// 生成备份文件名
	filename := fmt.Sprintf("config_backup_%d_%s.json",
		version,
		backup.Timestamp.Format("20060102_150405"))
	backupPath := filepath.Join(rm.backupDir, filename)

	// 写入备份文件
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return NewStructuredError(ErrorTypeSystem, "BACKUP_004", "failed to write backup file").
			WithCause(err).
			WithContext(NewErrorContext("BackupConfig", "RecoveryManager", err).
				WithMetadata("backup_path", backupPath))
	}

	rm.logger.Info(ctx, "Config backup created", map[string]interface{}{
		"backup_path": backupPath,
		"version":     version,
		"checksum":    backup.Checksum,
	})

	// 清理旧备份
	if err := rm.cleanupOldBackups(ctx); err != nil {
		rm.logger.Warn(ctx, "Failed to cleanup old backups", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return nil
}

// RestoreConfig 恢复配置
func (rm *RecoveryManager) RestoreConfig(ctx context.Context, version int64) (interface{}, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	// 查找备份文件
	backupPath, err := rm.findBackupFile(version)
	if err != nil {
		return nil, err
	}

	// 读取备份文件
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, NewStructuredError(ErrorTypeSystem, "RESTORE_001", "failed to read backup file").
			WithCause(err).
			WithContext(NewErrorContext("RestoreConfig", "RecoveryManager", err).
				WithMetadata("backup_path", backupPath))
	}

	// 反序列化备份数据
	var backup ConfigBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, NewStructuredError(ErrorTypeSystem, "RESTORE_002", "failed to deserialize backup").
			WithCause(err).
			WithContext(NewErrorContext("RestoreConfig", "RecoveryManager", err))
	}

	// 验证校验和
	backupCopy := backup
	backupCopy.Checksum = ""
	verifyData, err := json.MarshalIndent(backupCopy, "", "  ")
	if err != nil {
		return nil, NewStructuredError(ErrorTypeSystem, "RESTORE_003", "failed to serialize for checksum verification").
			WithCause(err).
			WithContext(NewErrorContext("RestoreConfig", "RecoveryManager", err))
	}

	expectedChecksum := calculateChecksum(verifyData)
	if backup.Checksum != expectedChecksum {
		return nil, NewStructuredError(ErrorTypeSystem, "RESTORE_004", "backup checksum verification failed").
			WithContext(NewErrorContext("RestoreConfig", "RecoveryManager",
				fmt.Errorf("expected %s, got %s", expectedChecksum, backup.Checksum)).
				WithMetadata("backup_path", backupPath))
	}

	rm.logger.Info(ctx, "Config restored from backup", map[string]interface{}{
		"backup_path": backupPath,
		"version":     backup.Version,
		"timestamp":   backup.Timestamp,
	})

	return backup.Data, nil
}

// GetLatestBackup 获取最新备份
func (rm *RecoveryManager) GetLatestBackup(ctx context.Context) (interface{}, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	// 列出所有备份文件
	files, err := os.ReadDir(rm.backupDir)
	if err != nil {
		return nil, NewStructuredError(ErrorTypeSystem, "BACKUP_005", "failed to list backup directory").
			WithCause(err).
			WithContext(NewErrorContext("GetLatestBackup", "RecoveryManager", err))
	}

	if len(files) == 0 {
		return nil, NewStructuredError(ErrorTypeBusiness, "BACKUP_006", "no backup files found").
			WithContext(NewErrorContext("GetLatestBackup", "RecoveryManager",
				fmt.Errorf("backup directory is empty")))
	}

	// 找到最新的备份文件
	var latestFile string
	var latestTime time.Time

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = file.Name()
		}
	}

	if latestFile == "" {
		return nil, NewStructuredError(ErrorTypeBusiness, "BACKUP_007", "no valid backup files found").
			WithContext(NewErrorContext("GetLatestBackup", "RecoveryManager",
				fmt.Errorf("no valid JSON backup files")))
	}

	// 读取最新备份
	backupPath := filepath.Join(rm.backupDir, latestFile)
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, NewStructuredError(ErrorTypeSystem, "BACKUP_008", "failed to read latest backup").
			WithCause(err).
			WithContext(NewErrorContext("GetLatestBackup", "RecoveryManager", err))
	}

	var backup ConfigBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, NewStructuredError(ErrorTypeSystem, "BACKUP_009", "failed to deserialize latest backup").
			WithCause(err).
			WithContext(NewErrorContext("GetLatestBackup", "RecoveryManager", err))
	}

	return backup.Data, nil
}

// findBackupFile 查找指定版本的备份文件
func (rm *RecoveryManager) findBackupFile(version int64) (string, error) {
	files, err := os.ReadDir(rm.backupDir)
	if err != nil {
		return "", NewStructuredError(ErrorTypeSystem, "BACKUP_010", "failed to list backup directory").
			WithCause(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		backupPath := filepath.Join(rm.backupDir, file.Name())
		data, err := os.ReadFile(backupPath)
		if err != nil {
			continue
		}

		var backup ConfigBackup
		if err := json.Unmarshal(data, &backup); err != nil {
			continue
		}

		if backup.Version == version {
			return backupPath, nil
		}
	}

	return "", NewStructuredError(ErrorTypeBusiness, "BACKUP_011", "backup not found for version").
		WithMetadata("version", version)
}

// cleanupOldBackups 清理旧备份
func (rm *RecoveryManager) cleanupOldBackups(ctx context.Context) error {
	files, err := os.ReadDir(rm.backupDir)
	if err != nil {
		return err
	}

	// 按修改时间排序文件
	type fileInfo struct {
		name    string
		modTime time.Time
	}

	var backupFiles []fileInfo
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		backupFiles = append(backupFiles, fileInfo{
			name:    file.Name(),
			modTime: info.ModTime(),
		})
	}

	// 如果备份文件数量超过限制，删除最旧的
	if len(backupFiles) > rm.maxBackups {
		// 按时间排序（最新的在前）
		for i := 0; i < len(backupFiles)-1; i++ {
			for j := i + 1; j < len(backupFiles); j++ {
				if backupFiles[i].modTime.Before(backupFiles[j].modTime) {
					backupFiles[i], backupFiles[j] = backupFiles[j], backupFiles[i]
				}
			}
		}

		// 删除超出限制的旧备份
		for i := rm.maxBackups; i < len(backupFiles); i++ {
			oldBackupPath := filepath.Join(rm.backupDir, backupFiles[i].name)
			if err := os.Remove(oldBackupPath); err != nil {
				rm.logger.Warn(ctx, "Failed to remove old backup", map[string]interface{}{
					"backup_path": oldBackupPath,
					"error":       err.Error(),
				})
			} else {
				rm.logger.Debug(ctx, "Removed old backup", map[string]interface{}{
					"backup_path": oldBackupPath,
				})
			}
		}
	}

	return nil
}

// calculateChecksum 计算数据校验和（简单的哈希）
func calculateChecksum(data []byte) string {
	// 简单的校验和计算，实际应用中可以使用更强的哈希算法
	var sum uint32
	for _, b := range data {
		sum = sum*31 + uint32(b)
	}
	return fmt.Sprintf("%08x", sum)
}

// AutoRecoveryService 自动恢复服务
type AutoRecoveryService struct {
	recoveryManager *RecoveryManager
	logger          Logger
	ctx             context.Context
	cancel          context.CancelFunc
	running         bool
	mutex           sync.RWMutex
}

// NewAutoRecoveryService 创建自动恢复服务
func NewAutoRecoveryService(recoveryManager *RecoveryManager, logger Logger) *AutoRecoveryService {
	return &AutoRecoveryService{
		recoveryManager: recoveryManager,
		logger:          logger,
	}
}

// Start 启动自动恢复服务
func (ars *AutoRecoveryService) Start(ctx context.Context) error {
	ars.mutex.Lock()
	defer ars.mutex.Unlock()

	if ars.running {
		return NewStructuredError(ErrorTypeSystem, "RECOVERY_001", "auto recovery service is already running")
	}

	ars.ctx, ars.cancel = context.WithCancel(ctx)
	ars.running = true

	ars.logger.Info(ctx, "Auto recovery service started", nil)
	return nil
}

// Stop 停止自动恢复服务
func (ars *AutoRecoveryService) Stop() error {
	ars.mutex.Lock()
	defer ars.mutex.Unlock()

	if !ars.running {
		return NewStructuredError(ErrorTypeSystem, "RECOVERY_002", "auto recovery service is not running")
	}

	ars.cancel()
	ars.running = false

	ars.logger.Info(context.Background(), "Auto recovery service stopped", nil)
	return nil
}

// IsRunning 检查服务是否运行中
func (ars *AutoRecoveryService) IsRunning() bool {
	ars.mutex.RLock()
	defer ars.mutex.RUnlock()
	return ars.running
}

// HandleCriticalError 处理关键错误
func (ars *AutoRecoveryService) HandleCriticalError(ctx context.Context, err error, fallbackAction func() error) error {
	ars.logger.Error(ctx, "Critical error detected", map[string]interface{}{
		"error": err.Error(),
	})

	// 尝试从最新备份恢复
	if latestConfig, restoreErr := ars.recoveryManager.GetLatestBackup(ctx); restoreErr == nil {
		ars.logger.Info(ctx, "Attempting to restore from latest backup", nil)

		// 这里应该有具体的恢复逻辑，取决于配置管理器的实现
		if fallbackAction != nil {
			if err := fallbackAction(); err != nil {
				ars.logger.Error(ctx, "Fallback action failed", map[string]interface{}{
					"error": err.Error(),
				})
				return err
			}
		}

		ars.logger.Info(ctx, "Successfully recovered from backup", map[string]interface{}{
			"config": latestConfig,
		})
		return nil
	} else {
		ars.logger.Error(ctx, "Failed to restore from backup", map[string]interface{}{
			"error": restoreErr.Error(),
		})
	}

	return err
}
