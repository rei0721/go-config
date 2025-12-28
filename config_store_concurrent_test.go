package config

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrentAccessOptimizer_CachedReads(t *testing.T) {
	defaultConfig := &TestConfig{Name: "cache_test", Value: 100, Enabled: true}
	store := NewConfigStore(defaultConfig, 5)
	optimizer := NewConcurrentAccessOptimizer(store)

	// 第一次读取（缓存未命中）
	config1, err := optimizer.GetConfigCached()
	if err != nil {
		t.Fatalf("Failed to get cached config: %v", err)
	}

	if config1.Name != "cache_test" {
		t.Errorf("Config name mismatch")
	}

	// 第二次读取（应该命中缓存）
	config2, err := optimizer.GetConfigCached()
	if err != nil {
		t.Fatalf("Failed to get cached config: %v", err)
	}

	if config2.Name != "cache_test" {
		t.Errorf("Cached config name mismatch")
	}

	// 检查统计信息
	stats := optimizer.GetStats()
	if stats.ReadHits == 0 {
		t.Errorf("Expected at least one cache hit")
	}

	if stats.ReadMisses == 0 {
		t.Errorf("Expected at least one cache miss")
	}
}

func TestConcurrentAccessOptimizer_BatchUpdates(t *testing.T) {
	defaultConfig := &TestConfig{Name: "batch_test", Value: 50, Enabled: false}
	store := NewConfigStore(defaultConfig, 5)
	optimizer := NewConcurrentAccessOptimizer(store)

	// 设置较短的批处理超时以便测试
	optimizer.SetBatchConfig(50*time.Millisecond, 3)

	// 添加多个更新到批处理队列
	err1 := optimizer.QueueUpdate(func(config *TestConfig) error {
		config.Value += 10
		return nil
	})

	err2 := optimizer.QueueUpdate(func(config *TestConfig) error {
		config.Enabled = true
		return nil
	})

	err3 := optimizer.QueueUpdate(func(config *TestConfig) error {
		config.Name = "batch_updated"
		return nil
	})

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("Failed to queue updates")
	}

	// 等待批处理执行
	time.Sleep(100 * time.Millisecond)

	// 验证更新结果
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Name != "batch_updated" || config.Value != 60 || !config.Enabled {
		t.Errorf("Batch update failed: Name=%s, Value=%d, Enabled=%v",
			config.Name, config.Value, config.Enabled)
	}

	// 检查统计信息
	stats := optimizer.GetStats()
	if stats.BatchWrites == 0 {
		t.Errorf("Expected at least one batch write")
	}
}

func TestConcurrentAccessOptimizer_ImmediateUpdate(t *testing.T) {
	defaultConfig := &TestConfig{Name: "immediate_test", Value: 25, Enabled: false}
	store := NewConfigStore(defaultConfig, 5)
	optimizer := NewConcurrentAccessOptimizer(store)

	// 执行立即更新
	err := optimizer.UpdateImmediate(func(config *TestConfig) error {
		config.Value = 75
		config.Enabled = true
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to perform immediate update: %v", err)
	}

	// 验证更新结果
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Value != 75 || !config.Enabled {
		t.Errorf("Immediate update failed: Value=%d, Enabled=%v", config.Value, config.Enabled)
	}

	// 检查统计信息
	stats := optimizer.GetStats()
	if stats.SingleWrites == 0 {
		t.Errorf("Expected at least one single write")
	}
}

func TestConcurrentAccessOptimizer_ReadOnlyView(t *testing.T) {
	defaultConfig := &TestConfig{Name: "readonly_test", Value: 35, Enabled: true}
	store := NewConfigStore(defaultConfig, 5)
	optimizer := NewConcurrentAccessOptimizer(store)

	// 创建只读视图
	view, err := optimizer.CreateReadOnlyView()
	if err != nil {
		t.Fatalf("Failed to create read-only view: %v", err)
	}

	// 验证视图内容
	config := view.GetConfig()
	if config.Name != "readonly_test" || config.Value != 35 || !config.Enabled {
		t.Errorf("Read-only view content mismatch")
	}

	// 验证版本号
	if view.GetVersion() != 0 {
		t.Errorf("Expected version 0, got %d", view.GetVersion())
	}

	// 验证创建时间
	if view.GetCreatedTime().IsZero() {
		t.Errorf("Created time should not be zero")
	}

	// 更新原始配置
	store.SetConfig(&TestConfig{Name: "updated", Value: 70, Enabled: false})

	// 验证只读视图没有受到影响
	viewConfig := view.GetConfig()
	if viewConfig.Name != "readonly_test" || viewConfig.Value != 35 {
		t.Errorf("Read-only view should not be affected by updates")
	}
}

func TestConcurrentAccessOptimizer_ConcurrentAccess(t *testing.T) {
	defaultConfig := &TestConfig{Name: "concurrent_test", Value: 0, Enabled: false}
	store := NewConfigStore(defaultConfig, 10)
	optimizer := NewConcurrentAccessOptimizer(store)

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// 启动多个goroutine进行并发读取
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := optimizer.GetConfigCached()
				if err != nil {
					t.Errorf("Concurrent read failed: %v", err)
				}
			}
		}()
	}

	// 启动goroutine进行并发写入
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations/10; j++ {
				err := optimizer.QueueUpdate(func(config *TestConfig) error {
					config.Value++
					return nil
				})
				if err != nil {
					t.Errorf("Concurrent write failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// 等待所有批处理完成
	optimizer.FlushPendingUpdates()

	// 验证最终状态
	config, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get final config: %v", err)
	}

	// 值应该被增加了多次
	if config.Value == 0 {
		t.Errorf("Expected value to be incremented, got %d", config.Value)
	}

	// 检查统计信息
	stats := optimizer.GetStats()
	if stats.ReadHits == 0 && stats.ReadMisses == 0 {
		t.Errorf("Expected some read operations")
	}
}

func TestConfigWatcher_ChangeNotification(t *testing.T) {
	defaultConfig := &TestConfig{Name: "watcher_test", Value: 10, Enabled: false}
	store := NewConfigStore(defaultConfig, 5)
	watcher := NewConfigWatcher(store)

	// 设置变更通知
	var notified bool
	var notifiedConfig *TestConfig
	var notifiedVersion int64

	watcher.AddCallback(func(config *TestConfig, version int64) {
		notified = true
		notifiedConfig = config
		notifiedVersion = version
	})

	// 启动监听器
	watcher.Start()
	defer watcher.Stop()

	// 等待监听器启动
	time.Sleep(50 * time.Millisecond)

	// 更新配置
	newConfig := &TestConfig{Name: "watcher_updated", Value: 20, Enabled: true}
	store.SetConfig(newConfig)

	// 等待通知
	time.Sleep(200 * time.Millisecond)

	// 验证通知
	if !notified {
		t.Errorf("Expected change notification")
	}

	if notifiedConfig == nil {
		t.Fatalf("Notified config is nil")
	}

	if notifiedConfig.Name != "watcher_updated" || notifiedConfig.Value != 20 {
		t.Errorf("Notified config mismatch")
	}

	if notifiedVersion != 1 {
		t.Errorf("Expected notified version 1, got %d", notifiedVersion)
	}
}
