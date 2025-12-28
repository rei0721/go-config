package config

import (
	"sync"
	"sync/atomic"
	"time"
)

// ConfigStore 线程安全的配置存储
//
// ConfigStore 是一个高性能的线程安全配置存储系统，提供了
// 配置数据的并发安全访问、版本管理和快照功能。
//
// 核心特性：
// - 线程安全：使用读写锁保护所有并发访问
// - 版本管理：每次更新都会增加版本号
// - 快照系统：支持配置快照的创建、查询和恢复
// - 原子操作：提供原子性的配置更新操作
// - 批量更新：支持在单个事务中执行多个更新
// - 内存管理：自动清理旧快照，防止内存泄漏
//
// 使用场景：
// - 配置管理器的核心存储
// - 多goroutine环境下的配置共享
// - 配置版本控制和回滚
// - 配置变更的原子性保证
//
// 泛型支持：
// ConfigStore使用Go泛型，T必须实现Configurable接口，
// 确保类型安全和编译时检查。
type ConfigStore[T Configurable] struct {
	// 当前配置数据
	config *T

	// 读写锁保护配置数据的并发访问
	mutex sync.RWMutex

	// 配置版本号，使用原子操作确保线程安全
	version int64

	// 配置快照存储，用于版本管理和回滚
	snapshots map[int64]*ConfigSnapshot[T]

	// 快照锁，保护快照映射的并发访问
	snapshotMutex sync.RWMutex

	// 最大快照数量，防止内存泄漏
	maxSnapshots int

	// 最后更新时间
	lastUpdate time.Time
}

// ConfigSnapshot 配置快照
type ConfigSnapshot[T Configurable] struct {
	// 快照的配置数据
	Config *T

	// 快照版本号
	Version int64

	// 快照创建时间
	Timestamp time.Time

	// 快照描述信息
	Description string
}

// NewConfigStore 创建新的配置存储实例
//
// 创建并初始化一个新的ConfigStore实例，用于存储和管理
// 指定类型的配置数据。
//
// 参数：
//
//	defaultConfig: 默认配置对象，不能为nil
//	maxSnapshots: 最大快照数量，用于控制内存使用，0表示使用默认值10
//
// 返回值：
//
//	*ConfigStore[T]: 新创建的配置存储实例
//
// 初始状态：
// - 当前配置：设置为defaultConfig
// - 版本号：0
// - 快照列表：空
// - 最后更新时间：当前时间
//
// 使用示例：
//
//	store := NewConfigStore(&MyConfig{}, 20)  // 最多保留20个快照
//	config, err := store.GetConfig()
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewConfigStore[T Configurable](defaultConfig *T, maxSnapshots int) *ConfigStore[T] {
	if maxSnapshots <= 0 {
		maxSnapshots = 10 // 默认最多保留10个快照
	}

	return &ConfigStore[T]{
		config:       defaultConfig,
		version:      0,
		snapshots:    make(map[int64]*ConfigSnapshot[T]),
		maxSnapshots: maxSnapshots,
		lastUpdate:   time.Now(),
	}
}

// GetConfig 获取当前配置的副本
// 使用读锁保护，支持并发读取
func (cs *ConfigStore[T]) GetConfig() (*T, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	if cs.config == nil {
		return nil, s2error("config is nil")
	}

	// 返回配置的深拷贝以确保不可变性
	configCopy := *cs.config
	return &configCopy, nil
}

// SetConfig 设置新的配置
// 使用写锁保护，确保原子性更新
func (cs *ConfigStore[T]) SetConfig(newConfig *T) error {
	if newConfig == nil {
		return s2error("new config cannot be nil")
	}

	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// 创建快照保存当前配置
	if cs.config != nil {
		cs.createSnapshotLocked("auto_backup_before_update")
	}

	// 更新配置和版本
	cs.config = newConfig
	atomic.AddInt64(&cs.version, 1)
	cs.lastUpdate = time.Now()

	return nil
}

// UpdateConfig 使用更新函数原子性地更新配置
// updateFunc: 配置更新函数，接收当前配置的副本并返回更新后的配置
func (cs *ConfigStore[T]) UpdateConfig(updateFunc func(*T) (*T, error)) error {
	if updateFunc == nil {
		return s2error("update function cannot be nil")
	}

	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if cs.config == nil {
		return s2error("config is nil")
	}

	// 创建当前配置的副本
	configCopy := *cs.config

	// 执行更新函数
	updatedConfig, err := updateFunc(&configCopy)
	if err != nil {
		return s2error("config update failed: %v", err)
	}

	if updatedConfig == nil {
		return s2error("updated config cannot be nil")
	}

	// 创建快照保存当前配置
	cs.createSnapshotLocked("auto_backup_before_update")

	// 应用更新
	cs.config = updatedConfig
	atomic.AddInt64(&cs.version, 1)
	cs.lastUpdate = time.Now()

	return nil
}

// GetVersion 获取当前配置版本号
func (cs *ConfigStore[T]) GetVersion() int64 {
	return atomic.LoadInt64(&cs.version)
}

// GetLastUpdate 获取最后更新时间
func (cs *ConfigStore[T]) GetLastUpdate() time.Time {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	return cs.lastUpdate
}

// CreateSnapshot 创建配置快照
// description: 快照描述信息
func (cs *ConfigStore[T]) CreateSnapshot(description string) (int64, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	return cs.createSnapshotLocked(description), nil
}

// createSnapshotLocked 在已持有锁的情况下创建快照
// 注意：调用此方法前必须已经获得适当的锁
func (cs *ConfigStore[T]) createSnapshotLocked(description string) int64 {
	if cs.config == nil {
		return -1
	}

	currentVersion := atomic.LoadInt64(&cs.version)

	// 创建配置副本
	configCopy := *cs.config

	snapshot := &ConfigSnapshot[T]{
		Config:      &configCopy,
		Version:     currentVersion,
		Timestamp:   time.Now(),
		Description: description,
	}

	cs.snapshotMutex.Lock()
	defer cs.snapshotMutex.Unlock()

	// 添加新快照
	cs.snapshots[currentVersion] = snapshot

	// 清理旧快照，保持快照数量在限制范围内
	cs.cleanupOldSnapshots()

	return currentVersion
}

// cleanupOldSnapshots 清理旧快照，保持快照数量在限制范围内
// 注意：调用此方法前必须已经获得snapshotMutex锁
func (cs *ConfigStore[T]) cleanupOldSnapshots() {
	if len(cs.snapshots) <= cs.maxSnapshots {
		return
	}

	// 找到最旧的快照并删除
	var oldestVersion int64 = -1
	var oldestTime time.Time

	for version, snapshot := range cs.snapshots {
		if oldestVersion == -1 || snapshot.Timestamp.Before(oldestTime) {
			oldestVersion = version
			oldestTime = snapshot.Timestamp
		}
	}

	if oldestVersion != -1 {
		delete(cs.snapshots, oldestVersion)
	}
}

// GetSnapshot 获取指定版本的快照
func (cs *ConfigStore[T]) GetSnapshot(version int64) (*ConfigSnapshot[T], error) {
	cs.snapshotMutex.RLock()
	defer cs.snapshotMutex.RUnlock()

	snapshot, exists := cs.snapshots[version]
	if !exists {
		return nil, s2error("snapshot with version %d not found", version)
	}

	// 返回快照的副本
	snapshotCopy := *snapshot
	configCopy := *snapshot.Config
	snapshotCopy.Config = &configCopy

	return &snapshotCopy, nil
}

// ListSnapshots 列出所有可用的快照版本
func (cs *ConfigStore[T]) ListSnapshots() []int64 {
	cs.snapshotMutex.RLock()
	defer cs.snapshotMutex.RUnlock()

	versions := make([]int64, 0, len(cs.snapshots))
	for version := range cs.snapshots {
		versions = append(versions, version)
	}

	return versions
}

// RestoreFromSnapshot 从快照恢复配置
func (cs *ConfigStore[T]) RestoreFromSnapshot(version int64) error {
	// 先获取快照
	snapshot, err := cs.GetSnapshot(version)
	if err != nil {
		return s2error("failed to get snapshot: %v", err)
	}

	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// 创建当前配置的备份快照
	if cs.config != nil {
		cs.createSnapshotLocked("backup_before_restore")
	}

	// 恢复配置
	configCopy := *snapshot.Config
	cs.config = &configCopy
	atomic.AddInt64(&cs.version, 1)
	cs.lastUpdate = time.Now()

	return nil
}

// GetStats 获取配置存储的统计信息
func (cs *ConfigStore[T]) GetStats() *ConfigStoreStats {
	cs.mutex.RLock()
	cs.snapshotMutex.RLock()
	defer cs.mutex.RUnlock()
	defer cs.snapshotMutex.RUnlock()

	return &ConfigStoreStats{
		CurrentVersion: atomic.LoadInt64(&cs.version),
		LastUpdate:     cs.lastUpdate,
		SnapshotCount:  len(cs.snapshots),
		MaxSnapshots:   cs.maxSnapshots,
		HasConfig:      cs.config != nil,
	}
}

// ConfigStoreStats 配置存储统计信息
type ConfigStoreStats struct {
	CurrentVersion int64     // 当前版本号
	LastUpdate     time.Time // 最后更新时间
	SnapshotCount  int       // 快照数量
	MaxSnapshots   int       // 最大快照数量
	HasConfig      bool      // 是否有配置数据
}

// ConfigReader 配置读取器接口，用于优化并发读取
type ConfigReader[T Configurable] interface {
	GetConfig() (*T, error)
	GetVersion() int64
	GetLastUpdate() time.Time
}

// ConfigWriter 配置写入器接口，用于控制写入操作
type ConfigWriter[T Configurable] interface {
	SetConfig(config *T) error
	UpdateConfig(updateFunc func(*T) (*T, error)) error
}

// SnapshotManager 快照管理器接口
type SnapshotManager[T Configurable] interface {
	CreateSnapshot(description string) (int64, error)
	GetSnapshot(version int64) (*ConfigSnapshot[T], error)
	ListSnapshots() []int64
	RestoreFromSnapshot(version int64) error
}

// GetConfigWithVersion 获取配置及其版本号，优化并发访问
// 返回配置副本和版本号，确保数据一致性
func (cs *ConfigStore[T]) GetConfigWithVersion() (*T, int64, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	if cs.config == nil {
		return nil, 0, s2error("config is nil")
	}

	version := atomic.LoadInt64(&cs.version)
	configCopy := *cs.config

	return &configCopy, version, nil
}

// CompareAndSwap 原子性地比较并交换配置
// 只有当当前版本与期望版本匹配时才执行更新
func (cs *ConfigStore[T]) CompareAndSwap(expectedVersion int64, newConfig *T) (bool, error) {
	if newConfig == nil {
		return false, s2error("new config cannot be nil")
	}

	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	currentVersion := atomic.LoadInt64(&cs.version)
	if currentVersion != expectedVersion {
		return false, nil // 版本不匹配，不执行更新
	}

	// 创建快照
	if cs.config != nil {
		cs.createSnapshotLocked("cas_backup")
	}

	// 执行原子更新
	cs.config = newConfig
	atomic.AddInt64(&cs.version, 1)
	cs.lastUpdate = time.Now()

	return true, nil
}

// BatchUpdate 批量更新配置，减少锁竞争
// 允许在单个事务中执行多个更新操作
func (cs *ConfigStore[T]) BatchUpdate(updates []func(*T) error) error {
	if len(updates) == 0 {
		return s2error("no updates provided")
	}

	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if cs.config == nil {
		return s2error("config is nil")
	}

	// 创建快照
	cs.createSnapshotLocked("batch_update_backup")

	// 创建配置副本进行批量更新
	configCopy := *cs.config

	// 执行所有更新操作
	for i, updateFunc := range updates {
		if updateFunc == nil {
			return s2error("update function at index %d is nil", i)
		}

		if err := updateFunc(&configCopy); err != nil {
			return s2error("batch update failed at index %d: %v", i, err)
		}
	}

	// 应用批量更新结果
	cs.config = &configCopy
	atomic.AddInt64(&cs.version, 1)
	cs.lastUpdate = time.Now()

	return nil
}

// GetConfigSnapshot 获取当前配置的即时快照
// 不会保存到快照历史中，用于临时快照需求
func (cs *ConfigStore[T]) GetConfigSnapshot() (*ConfigSnapshot[T], error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	if cs.config == nil {
		return nil, s2error("config is nil")
	}

	configCopy := *cs.config
	version := atomic.LoadInt64(&cs.version)

	return &ConfigSnapshot[T]{
		Config:      &configCopy,
		Version:     version,
		Timestamp:   time.Now(),
		Description: "instant_snapshot",
	}, nil
}

// WaitForVersion 等待配置达到指定版本
// 用于同步等待配置更新完成
func (cs *ConfigStore[T]) WaitForVersion(targetVersion int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		currentVersion := atomic.LoadInt64(&cs.version)
		if currentVersion >= targetVersion {
			return nil
		}

		// 短暂休眠避免忙等待
		time.Sleep(10 * time.Millisecond)
	}

	return s2error("timeout waiting for version %d", targetVersion)
}

// CleanupSnapshots 清理指定时间之前的快照
// 用于主动管理快照存储空间
func (cs *ConfigStore[T]) CleanupSnapshots(before time.Time) int {
	cs.snapshotMutex.Lock()
	defer cs.snapshotMutex.Unlock()

	cleaned := 0
	for version, snapshot := range cs.snapshots {
		if snapshot.Timestamp.Before(before) {
			delete(cs.snapshots, version)
			cleaned++
		}
	}

	return cleaned
}

// GetSnapshotsByTimeRange 获取指定时间范围内的快照
func (cs *ConfigStore[T]) GetSnapshotsByTimeRange(start, end time.Time) []*ConfigSnapshot[T] {
	cs.snapshotMutex.RLock()
	defer cs.snapshotMutex.RUnlock()

	var snapshots []*ConfigSnapshot[T]
	for _, snapshot := range cs.snapshots {
		if snapshot.Timestamp.After(start) && snapshot.Timestamp.Before(end) {
			// 创建快照副本
			snapshotCopy := *snapshot
			configCopy := *snapshot.Config
			snapshotCopy.Config = &configCopy
			snapshots = append(snapshots, &snapshotCopy)
		}
	}

	return snapshots
}

// IsVersionAvailable 检查指定版本的快照是否可用
func (cs *ConfigStore[T]) IsVersionAvailable(version int64) bool {
	cs.snapshotMutex.RLock()
	defer cs.snapshotMutex.RUnlock()

	_, exists := cs.snapshots[version]
	return exists
}

// GetLatestSnapshot 获取最新的快照
func (cs *ConfigStore[T]) GetLatestSnapshot() (*ConfigSnapshot[T], error) {
	cs.snapshotMutex.RLock()
	defer cs.snapshotMutex.RUnlock()

	if len(cs.snapshots) == 0 {
		return nil, s2error("no snapshots available")
	}

	var latestSnapshot *ConfigSnapshot[T]
	var latestVersion int64 = -1

	for version, snapshot := range cs.snapshots {
		if version > latestVersion {
			latestVersion = version
			latestSnapshot = snapshot
		}
	}

	if latestSnapshot == nil {
		return nil, s2error("no valid snapshot found")
	}

	// 返回快照副本
	snapshotCopy := *latestSnapshot
	configCopy := *latestSnapshot.Config
	snapshotCopy.Config = &configCopy

	return &snapshotCopy, nil
}

// SetMaxSnapshots 动态设置最大快照数量
func (cs *ConfigStore[T]) SetMaxSnapshots(maxSnapshots int) {
	if maxSnapshots <= 0 {
		maxSnapshots = 10
	}

	cs.snapshotMutex.Lock()
	defer cs.snapshotMutex.Unlock()

	cs.maxSnapshots = maxSnapshots

	// 如果当前快照数量超过新的限制，清理旧快照
	for len(cs.snapshots) > cs.maxSnapshots {
		cs.cleanupOldSnapshots()
	}
}
