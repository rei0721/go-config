package config

import (
	"context"
	"sync"
	"time"
)

// ConcurrentAccessOptimizer 并发访问优化器
// 提供读写分离、缓存和批处理等优化功能
type ConcurrentAccessOptimizer[T Configurable] struct {
	store *ConfigStore[T]

	// 读缓存，减少锁竞争
	readCache     *T
	cacheVersion  int64
	cacheMutex    sync.RWMutex
	cacheTimeout  time.Duration
	lastCacheTime time.Time

	// 写操作批处理
	writeBatch   []func(*T) error
	batchMutex   sync.Mutex
	batchTimeout time.Duration
	batchTimer   *time.Timer
	maxBatchSize int

	// 统计信息
	stats *ConcurrentStats
}

// ConcurrentStats 并发访问统计信息
type ConcurrentStats struct {
	ReadHits     int64 // 缓存命中次数
	ReadMisses   int64 // 缓存未命中次数
	BatchWrites  int64 // 批处理写入次数
	SingleWrites int64 // 单独写入次数
	CacheRefresh int64 // 缓存刷新次数
	mutex        sync.RWMutex
}

// NewConcurrentAccessOptimizer 创建并发访问优化器
func NewConcurrentAccessOptimizer[T Configurable](store *ConfigStore[T]) *ConcurrentAccessOptimizer[T] {
	return &ConcurrentAccessOptimizer[T]{
		store:        store,
		cacheTimeout: 100 * time.Millisecond, // 默认缓存100ms
		batchTimeout: 50 * time.Millisecond,  // 默认批处理50ms
		maxBatchSize: 10,                     // 默认最大批处理10个操作
		stats:        &ConcurrentStats{},
	}
}

// GetConfigCached 获取缓存的配置，优化频繁读取
func (cao *ConcurrentAccessOptimizer[T]) GetConfigCached() (*T, error) {
	// 首先检查缓存
	cao.cacheMutex.RLock()
	if cao.readCache != nil &&
		time.Since(cao.lastCacheTime) < cao.cacheTimeout &&
		cao.cacheVersion == cao.store.GetVersion() {

		// 缓存命中
		configCopy := *cao.readCache
		cao.cacheMutex.RUnlock()

		cao.stats.mutex.Lock()
		cao.stats.ReadHits++
		cao.stats.mutex.Unlock()

		return &configCopy, nil
	}
	cao.cacheMutex.RUnlock()

	// 缓存未命中，从存储获取
	config, version, err := cao.store.GetConfigWithVersion()
	if err != nil {
		cao.stats.mutex.Lock()
		cao.stats.ReadMisses++
		cao.stats.mutex.Unlock()
		return nil, err
	}

	// 更新缓存
	cao.cacheMutex.Lock()
	cao.readCache = config
	cao.cacheVersion = version
	cao.lastCacheTime = time.Now()
	cao.cacheMutex.Unlock()

	cao.stats.mutex.Lock()
	cao.stats.ReadMisses++
	cao.stats.CacheRefresh++
	cao.stats.mutex.Unlock()

	configCopy := *config
	return &configCopy, nil
}

// InvalidateCache 使缓存失效
func (cao *ConcurrentAccessOptimizer[T]) InvalidateCache() {
	cao.cacheMutex.Lock()
	defer cao.cacheMutex.Unlock()

	cao.readCache = nil
	cao.cacheVersion = -1
	cao.lastCacheTime = time.Time{}
}

// QueueUpdate 将更新操作加入批处理队列
func (cao *ConcurrentAccessOptimizer[T]) QueueUpdate(updateFunc func(*T) error) error {
	if updateFunc == nil {
		return s2error("update function cannot be nil")
	}

	cao.batchMutex.Lock()
	defer cao.batchMutex.Unlock()

	// 添加到批处理队列
	cao.writeBatch = append(cao.writeBatch, updateFunc)

	// 如果达到最大批处理大小，立即执行
	if len(cao.writeBatch) >= cao.maxBatchSize {
		return cao.flushBatch()
	}

	// 设置或重置批处理定时器
	if cao.batchTimer != nil {
		cao.batchTimer.Stop()
	}

	cao.batchTimer = time.AfterFunc(cao.batchTimeout, func() {
		cao.batchMutex.Lock()
		defer cao.batchMutex.Unlock()
		cao.flushBatch()
	})

	return nil
}

// flushBatch 执行批处理写入
// 注意：调用此方法前必须已经获得batchMutex锁
func (cao *ConcurrentAccessOptimizer[T]) flushBatch() error {
	if len(cao.writeBatch) == 0 {
		return nil
	}

	// 执行批量更新
	err := cao.store.BatchUpdate(cao.writeBatch)

	// 清空批处理队列
	cao.writeBatch = cao.writeBatch[:0]

	// 停止定时器
	if cao.batchTimer != nil {
		cao.batchTimer.Stop()
		cao.batchTimer = nil
	}

	// 使缓存失效
	cao.InvalidateCache()

	// 更新统计信息
	cao.stats.mutex.Lock()
	if err == nil {
		cao.stats.BatchWrites++
	}
	cao.stats.mutex.Unlock()

	return err
}

// UpdateImmediate 立即执行更新操作，不使用批处理
func (cao *ConcurrentAccessOptimizer[T]) UpdateImmediate(updateFunc func(*T) error) error {
	err := cao.store.UpdateConfig(func(config *T) (*T, error) {
		err := updateFunc(config)
		return config, err
	})

	if err == nil {
		// 使缓存失效
		cao.InvalidateCache()

		// 更新统计信息
		cao.stats.mutex.Lock()
		cao.stats.SingleWrites++
		cao.stats.mutex.Unlock()
	}

	return err
}

// FlushPendingUpdates 强制执行所有待处理的更新
func (cao *ConcurrentAccessOptimizer[T]) FlushPendingUpdates() error {
	cao.batchMutex.Lock()
	defer cao.batchMutex.Unlock()

	return cao.flushBatch()
}

// GetStats 获取并发访问统计信息
func (cao *ConcurrentAccessOptimizer[T]) GetStats() ConcurrentStats {
	cao.stats.mutex.RLock()
	defer cao.stats.mutex.RUnlock()

	// 返回统计信息的副本，不包含mutex
	return ConcurrentStats{
		ReadHits:     cao.stats.ReadHits,
		ReadMisses:   cao.stats.ReadMisses,
		BatchWrites:  cao.stats.BatchWrites,
		SingleWrites: cao.stats.SingleWrites,
		CacheRefresh: cao.stats.CacheRefresh,
	}
}

// ResetStats 重置统计信息
func (cao *ConcurrentAccessOptimizer[T]) ResetStats() {
	cao.stats.mutex.Lock()
	defer cao.stats.mutex.Unlock()

	cao.stats.ReadHits = 0
	cao.stats.ReadMisses = 0
	cao.stats.BatchWrites = 0
	cao.stats.SingleWrites = 0
	cao.stats.CacheRefresh = 0
}

// SetCacheTimeout 设置缓存超时时间
func (cao *ConcurrentAccessOptimizer[T]) SetCacheTimeout(timeout time.Duration) {
	cao.cacheMutex.Lock()
	defer cao.cacheMutex.Unlock()

	cao.cacheTimeout = timeout
}

// SetBatchConfig 设置批处理配置
func (cao *ConcurrentAccessOptimizer[T]) SetBatchConfig(timeout time.Duration, maxSize int) {
	cao.batchMutex.Lock()
	defer cao.batchMutex.Unlock()

	cao.batchTimeout = timeout
	cao.maxBatchSize = maxSize
}

// ReadOnlyView 创建只读视图，用于安全的并发读取
type ReadOnlyView[T Configurable] struct {
	config  *T
	version int64
	created time.Time
}

// GetConfig 获取只读配置
func (rov *ReadOnlyView[T]) GetConfig() *T {
	return rov.config
}

// GetVersion 获取配置版本
func (rov *ReadOnlyView[T]) GetVersion() int64 {
	return rov.version
}

// GetCreatedTime 获取视图创建时间
func (rov *ReadOnlyView[T]) GetCreatedTime() time.Time {
	return rov.created
}

// CreateReadOnlyView 创建配置的只读视图
func (cao *ConcurrentAccessOptimizer[T]) CreateReadOnlyView() (*ReadOnlyView[T], error) {
	config, version, err := cao.store.GetConfigWithVersion()
	if err != nil {
		return nil, err
	}

	return &ReadOnlyView[T]{
		config:  config,
		version: version,
		created: time.Now(),
	}, nil
}

// ConfigWatcher 配置变更监听器
type ConfigWatcher[T Configurable] struct {
	store     *ConfigStore[T]
	callbacks []func(*T, int64)
	mutex     sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
}

// NewConfigWatcher 创建配置变更监听器
func NewConfigWatcher[T Configurable](store *ConfigStore[T]) *ConfigWatcher[T] {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher[T]{
		store:  store,
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddCallback 添加变更回调函数
func (cw *ConfigWatcher[T]) AddCallback(callback func(*T, int64)) {
	if callback == nil {
		return
	}

	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	cw.callbacks = append(cw.callbacks, callback)
}

// Start 启动监听器
func (cw *ConfigWatcher[T]) Start() {
	cw.mutex.Lock()
	if cw.running {
		cw.mutex.Unlock()
		return
	}
	cw.running = true
	cw.mutex.Unlock()

	go cw.watchLoop()
}

// Stop 停止监听器
func (cw *ConfigWatcher[T]) Stop() {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	if !cw.running {
		return
	}

	cw.running = false
	cw.cancel()
}

// watchLoop 监听循环
func (cw *ConfigWatcher[T]) watchLoop() {
	lastVersion := cw.store.GetVersion()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cw.ctx.Done():
			return
		case <-ticker.C:
			currentVersion := cw.store.GetVersion()
			if currentVersion != lastVersion {
				config, _, err := cw.store.GetConfigWithVersion()
				if err == nil {
					cw.notifyCallbacks(config, currentVersion)
				}
				lastVersion = currentVersion
			}
		}
	}
}

// notifyCallbacks 通知所有回调函数
func (cw *ConfigWatcher[T]) notifyCallbacks(config *T, version int64) {
	cw.mutex.RLock()
	callbacks := make([]func(*T, int64), len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mutex.RUnlock()

	for _, callback := range callbacks {
		go func(cb func(*T, int64)) {
			defer func() {
				if r := recover(); r != nil {
					// 忽略回调函数中的panic，避免影响其他回调
				}
			}()
			cb(config, version)
		}(callback)
	}
}
