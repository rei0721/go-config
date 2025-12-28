package config

import (
	"testing"
)

// TestConfig 测试配置结构
type TestConfig struct {
	Name    string `yaml:"name"`
	Value   int    `yaml:"value"`
	Enabled bool   `yaml:"enabled"`
}

func TestConfigStore_BasicOperations(t *testing.T) {
	// 创建默认配置
	defaultConfig := &TestConfig{
		Name:    "test",
		Value:   42,
		Enabled: true,
	}

	// 创建配置存储
	store := NewConfigStore(defaultConfig, 5)

	// 测试获取配置
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Name != "test" || config.Value != 42 || !config.Enabled {
		t.Errorf("Config values don't match expected values")
	}

	// 测试版本号
	version := store.GetVersion()
	if version != 0 {
		t.Errorf("Expected initial version 0, got %d", version)
	}
}

func TestConfigStore_SetConfig(t *testing.T) {
	defaultConfig := &TestConfig{Name: "initial", Value: 1, Enabled: false}
	store := NewConfigStore(defaultConfig, 5)

	// 设置新配置
	newConfig := &TestConfig{Name: "updated", Value: 2, Enabled: true}
	err := store.SetConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	// 验证配置已更新
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Name != "updated" || config.Value != 2 || !config.Enabled {
		t.Errorf("Config was not updated correctly")
	}

	// 验证版本号已增加
	version := store.GetVersion()
	if version != 1 {
		t.Errorf("Expected version 1, got %d", version)
	}
}

func TestConfigStore_UpdateConfig(t *testing.T) {
	defaultConfig := &TestConfig{Name: "test", Value: 10, Enabled: false}
	store := NewConfigStore(defaultConfig, 5)

	// 使用更新函数
	err := store.UpdateConfig(func(config *TestConfig) (*TestConfig, error) {
		config.Value = config.Value * 2
		config.Enabled = true
		return config, nil
	})

	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// 验证更新结果
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Value != 20 || !config.Enabled {
		t.Errorf("Config update failed: Value=%d, Enabled=%v", config.Value, config.Enabled)
	}
}

func TestConfigStore_Snapshots(t *testing.T) {
	defaultConfig := &TestConfig{Name: "snapshot_test", Value: 100, Enabled: true}
	store := NewConfigStore(defaultConfig, 3)

	// 创建快照
	version, err := store.CreateSnapshot("initial_snapshot")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if version != 0 {
		t.Errorf("Expected snapshot version 0, got %d", version)
	}

	// 更新配置
	newConfig := &TestConfig{Name: "updated", Value: 200, Enabled: false}
	store.SetConfig(newConfig)

	// 从快照恢复
	err = store.RestoreFromSnapshot(version)
	if err != nil {
		t.Fatalf("Failed to restore from snapshot: %v", err)
	}

	// 验证恢复结果
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Name != "snapshot_test" || config.Value != 100 || !config.Enabled {
		t.Errorf("Snapshot restore failed")
	}
}

func TestConfigStore_GetConfigWithVersion(t *testing.T) {
	defaultConfig := &TestConfig{Name: "version_test", Value: 50, Enabled: true}
	store := NewConfigStore(defaultConfig, 5)

	// 获取配置和版本
	config, version, err := store.GetConfigWithVersion()
	if err != nil {
		t.Fatalf("Failed to get config with version: %v", err)
	}

	if version != 0 {
		t.Errorf("Expected version 0, got %d", version)
	}

	if config.Name != "version_test" {
		t.Errorf("Config name mismatch")
	}

	// 更新配置
	store.SetConfig(&TestConfig{Name: "updated", Value: 75, Enabled: false})

	// 再次获取
	config2, version2, err := store.GetConfigWithVersion()
	if err != nil {
		t.Fatalf("Failed to get updated config with version: %v", err)
	}

	if version2 != 1 {
		t.Errorf("Expected version 1, got %d", version2)
	}

	if config2.Name != "updated" {
		t.Errorf("Updated config name mismatch")
	}
}

func TestConfigStore_CompareAndSwap(t *testing.T) {
	defaultConfig := &TestConfig{Name: "cas_test", Value: 30, Enabled: false}
	store := NewConfigStore(defaultConfig, 5)

	currentVersion := store.GetVersion()

	// 成功的CAS操作
	newConfig := &TestConfig{Name: "cas_updated", Value: 60, Enabled: true}
	success, err := store.CompareAndSwap(currentVersion, newConfig)
	if err != nil {
		t.Fatalf("CAS operation failed: %v", err)
	}

	if !success {
		t.Errorf("Expected CAS to succeed")
	}

	// 失败的CAS操作（版本不匹配）
	anotherConfig := &TestConfig{Name: "should_fail", Value: 90, Enabled: false}
	success2, err := store.CompareAndSwap(currentVersion, anotherConfig) // 使用旧版本号
	if err != nil {
		t.Fatalf("CAS operation failed: %v", err)
	}

	if success2 {
		t.Errorf("Expected CAS to fail due to version mismatch")
	}

	// 验证配置没有被错误更新
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Name != "cas_updated" {
		t.Errorf("Config was incorrectly updated by failed CAS")
	}
}

func TestConfigStore_Stats(t *testing.T) {
	defaultConfig := &TestConfig{Name: "stats_test", Value: 15, Enabled: true}
	store := NewConfigStore(defaultConfig, 5)

	// 获取初始统计信息
	stats := store.GetStats()
	if stats.CurrentVersion != 0 {
		t.Errorf("Expected initial version 0, got %d", stats.CurrentVersion)
	}

	if !stats.HasConfig {
		t.Errorf("Expected HasConfig to be true")
	}

	if stats.SnapshotCount != 0 {
		t.Errorf("Expected initial snapshot count 0, got %d", stats.SnapshotCount)
	}

	// 创建快照并更新配置
	store.CreateSnapshot("test_snapshot")
	store.SetConfig(&TestConfig{Name: "updated", Value: 25, Enabled: false})

	// 获取更新后的统计信息
	stats2 := store.GetStats()
	if stats2.CurrentVersion != 1 {
		t.Errorf("Expected version 1, got %d", stats2.CurrentVersion)
	}

	if stats2.SnapshotCount != 1 {
		t.Errorf("Expected snapshot count 1, got %d", stats2.SnapshotCount)
	}

	// 验证最后更新时间
	if stats2.LastUpdate.IsZero() {
		t.Errorf("LastUpdate should not be zero")
	}

	if stats2.LastUpdate.Before(stats.LastUpdate) {
		t.Errorf("LastUpdate should be more recent")
	}
}
