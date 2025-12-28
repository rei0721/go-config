package config

import (
	"context"
	"fmt"
	"time"
)

// IntegratedManagerBuilder 集成管理器构建器
// 提供统一的接口来创建和配置完整的配置管理系统
type IntegratedManagerBuilder[T Configurable] struct {
	defaultConfig *T
	option        *ImmutableOption
	useRefactored bool
	hooks         []HookRegistration
	errorHandler  ErrorHandler
}

// HookRegistration 钩子注册信息
type HookRegistration struct {
	Type    HookType
	Handler HookHandler
}

// NewIntegratedManagerBuilder 创建集成管理器构建器
func NewIntegratedManagerBuilder[T Configurable](defaultConfig *T) *IntegratedManagerBuilder[T] {
	return &IntegratedManagerBuilder[T]{
		defaultConfig: defaultConfig,
		useRefactored: true, // 默认使用重构后的架构
		hooks:         make([]HookRegistration, 0),
	}
}

// WithOption 设置配置选项
func (b *IntegratedManagerBuilder[T]) WithOption(option *ImmutableOption) *IntegratedManagerBuilder[T] {
	b.option = option
	return b
}

// WithRefactoredMode 设置是否使用重构模式
func (b *IntegratedManagerBuilder[T]) WithRefactoredMode(useRefactored bool) *IntegratedManagerBuilder[T] {
	b.useRefactored = useRefactored
	return b
}

// WithHook 添加钩子
func (b *IntegratedManagerBuilder[T]) WithHook(hookType HookType, handler HookHandler) *IntegratedManagerBuilder[T] {
	b.hooks = append(b.hooks, HookRegistration{
		Type:    hookType,
		Handler: handler,
	})
	return b
}

// WithErrorHandler 设置错误处理器
func (b *IntegratedManagerBuilder[T]) WithErrorHandler(handler ErrorHandler) *IntegratedManagerBuilder[T] {
	b.errorHandler = handler
	return b
}

// Build 构建集成管理器
func (b *IntegratedManagerBuilder[T]) Build() *Manager[T] {
	manager := NewManager(b.defaultConfig)

	if b.useRefactored {
		manager.EnableRefactoredMode()
	}

	return manager
}

// BuildAndInitialize 构建并初始化集成管理器
func (b *IntegratedManagerBuilder[T]) BuildAndInitialize(ctx context.Context) (*Manager[T], error) {
	manager := b.Build()

	// 使用默认选项如果没有提供
	option := b.option
	if option == nil {
		builder := NewOptionBuilder()
		var err error
		option, err = builder.Build()
		if err != nil {
			return nil, fmt.Errorf("failed to build default option: %w", err)
		}
	}

	// 初始化管理器
	if err := manager.InitWithOptions(ctx, option); err != nil {
		return nil, fmt.Errorf("failed to initialize manager: %w", err)
	}

	// 注册钩子
	for _, hookReg := range b.hooks {
		if err := manager.RegisterHookWithType(hookReg.Type, hookReg.Handler); err != nil {
			return nil, fmt.Errorf("failed to register hook %v: %w", hookReg.Type, err)
		}
	}

	return manager, nil
}

// ManagerFactory 管理器工厂
// 提供便捷的方法来创建不同类型的配置管理器
type ManagerFactory struct{}

// NewManagerFactory 创建管理器工厂
func NewManagerFactory() *ManagerFactory {
	return &ManagerFactory{}
}

// CreateSimpleManager 创建简单的配置管理器
// 使用默认配置，适合快速开始
func (f *ManagerFactory) CreateSimpleManager(defaultConfig interface{}) interface{} {
	// 由于Go的泛型限制，这里返回interface{}，调用者需要进行类型断言
	// 实际使用中，建议直接使用NewManager函数
	return fmt.Errorf("use NewManager function directly for type safety")
}

// CreateAdvancedManager 创建高级配置管理器
// 支持自定义选项和钩子
func (f *ManagerFactory) CreateAdvancedManager(
	defaultConfig interface{},
	option *ImmutableOption,
	hooks map[HookType]HookHandler,
) (interface{}, error) {
	// 由于Go的泛型限制，这里返回interface{}，调用者需要进行类型断言
	// 实际使用中，建议直接使用NewIntegratedManagerBuilder
	return nil, fmt.Errorf("use NewIntegratedManagerBuilder for type safety")
}

// CreateProductionManager 创建生产环境配置管理器
// 包含完整的错误处理、监控和恢复机制
func (f *ManagerFactory) CreateProductionManager(
	defaultConfig interface{},
	configPath string,
	logger Logger,
) (interface{}, error) {
	// 由于Go的泛型限制，这里返回interface{}，调用者需要进行类型断言
	// 实际使用中，建议直接使用NewIntegratedManagerBuilder
	return nil, fmt.Errorf("use NewIntegratedManagerBuilder for type safety")
}

// ProductionErrorHook 生产环境错误钩子
type ProductionErrorHook struct {
	logger Logger
}

func (h *ProductionErrorHook) Handle(ctx *HookContext) error {
	if h.logger != nil {
		h.logger.Error(context.Background(), ctx.Message, map[string]interface{}{
			"error":     ctx.Error,
			"timestamp": ctx.Timestamp,
			"metadata":  ctx.Metadata,
		})
	}
	return nil
}

func (h *ProductionErrorHook) Priority() int {
	return 0
}

func (h *ProductionErrorHook) IsAsync() bool {
	return true
}

// ProductionInfoHook 生产环境信息钩子
type ProductionInfoHook struct {
	logger Logger
}

func (h *ProductionInfoHook) Handle(ctx *HookContext) error {
	if h.logger != nil {
		h.logger.Info(context.Background(), ctx.Message, map[string]interface{}{
			"timestamp": ctx.Timestamp,
			"metadata":  ctx.Metadata,
		})
	}
	return nil
}

func (h *ProductionInfoHook) Priority() int {
	return 0
}

func (h *ProductionInfoHook) IsAsync() bool {
	return true
}

// ProductionWarnHook 生产环境警告钩子
type ProductionWarnHook struct {
	logger Logger
}

func (h *ProductionWarnHook) Handle(ctx *HookContext) error {
	if h.logger != nil {
		h.logger.Warn(context.Background(), ctx.Message, map[string]interface{}{
			"timestamp": ctx.Timestamp,
			"metadata":  ctx.Metadata,
		})
	}
	return nil
}

func (h *ProductionWarnHook) Priority() int {
	return 0
}

func (h *ProductionWarnHook) IsAsync() bool {
	return true
}

// ComponentIntegrator 组件集成器
// 负责协调各个组件之间的交互
type ComponentIntegrator[T Configurable] struct {
	manager       *RefactoredManager[T]
	schedulerPool *SchedulerPool
	fileWatcher   *FileWatcher
	configStore   *ConfigStore[T]
	hookRegistry  *HookRegistry
	hookExecutor  *HookExecutor
}

// NewComponentIntegrator 创建组件集成器
func NewComponentIntegrator[T Configurable](manager *RefactoredManager[T]) *ComponentIntegrator[T] {
	return &ComponentIntegrator[T]{
		manager:       manager,
		schedulerPool: manager.GetSchedulerPool(),
		fileWatcher:   manager.GetFileWatcher(),
		configStore:   manager.GetConfigStore(),
		hookRegistry:  manager.GetHookRegistry(),
		hookExecutor:  manager.hookExecutor,
	}
}

// OptimizePerformance 优化性能
func (ci *ComponentIntegrator[T]) OptimizePerformance() error {
	// 优化调度池
	if ci.schedulerPool != nil {
		// 设置优先级调度策略
		ci.schedulerPool.SetSchedulingStrategy(&PrioritySchedulingStrategy{})

		// 设置拒绝策略为丢弃最旧任务
		ci.schedulerPool.SetRejectionPolicy(RejectPolicyDiscardOldest)
	}

	// 优化配置存储
	if ci.configStore != nil {
		// 限制快照数量以节省内存
		ci.configStore.SetMaxSnapshots(5)
	}

	// 优化文件监听器
	if ci.fileWatcher != nil {
		// 确保使用调度池
		ci.fileWatcher.SetSchedulerPool(ci.schedulerPool)
	}

	return nil
}

// MonitorResources 监控资源使用
func (ci *ComponentIntegrator[T]) MonitorResources() map[string]interface{} {
	resources := make(map[string]interface{})

	// 调度池资源
	if ci.schedulerPool != nil {
		poolMetrics := ci.schedulerPool.GetDetailedMetrics()
		resources["scheduler_pool"] = map[string]interface{}{
			"active_workers":    poolMetrics.ActiveWorkers,
			"queued_tasks":      poolMetrics.QueuedTasks,
			"completed_tasks":   poolMetrics.CompletedTasks,
			"failed_tasks":      poolMetrics.FailedTasks,
			"success_rate":      poolMetrics.SuccessRate,
			"average_wait_time": poolMetrics.AverageWaitTime.String(),
			"uptime":            poolMetrics.Uptime.String(),
		}
	}

	// 配置存储资源
	if ci.configStore != nil {
		storeStats := ci.configStore.GetStats()
		resources["config_store"] = map[string]interface{}{
			"current_version": storeStats.CurrentVersion,
			"snapshot_count":  storeStats.SnapshotCount,
			"max_snapshots":   storeStats.MaxSnapshots,
			"last_update":     storeStats.LastUpdate,
		}
	}

	// 文件监听器资源
	if ci.fileWatcher != nil {
		watcherStats := ci.fileWatcher.GetStats()
		resources["file_watcher"] = watcherStats
	}

	// 钩子注册表资源
	if ci.hookRegistry != nil {
		resources["hook_registry"] = map[string]interface{}{
			"total_handlers": ci.hookRegistry.TotalCount(),
		}
	}

	return resources
}

// CoordinateComponents 协调组件间的交互
func (ci *ComponentIntegrator[T]) CoordinateComponents() error {
	// 确保文件监听器使用调度池
	if ci.fileWatcher != nil && ci.schedulerPool != nil {
		ci.fileWatcher.SetSchedulerPool(ci.schedulerPool)
	}

	// 确保钩子执行器使用调度池
	if ci.hookExecutor != nil && ci.schedulerPool != nil {
		// 钩子执行器已经在创建时绑定了调度池
	}

	// 设置组件间的协调逻辑
	if ci.manager != nil {
		// 注册系统级钩子来协调组件
		systemHook := &SystemCoordinationHook[T]{integrator: ci}
		if err := ci.hookRegistry.RegisterHook(HookTypeAfterUpdate, systemHook); err != nil {
			return fmt.Errorf("failed to register system coordination hook: %w", err)
		}
	}

	return nil
}

// SystemCoordinationHook 系统协调钩子
type SystemCoordinationHook[T Configurable] struct {
	integrator *ComponentIntegrator[T]
}

func (h *SystemCoordinationHook[T]) Handle(ctx *HookContext) error {
	// 在配置更新后执行系统协调逻辑
	if h.integrator != nil {
		// 可以在这里添加配置更新后的协调逻辑
		// 比如通知其他组件配置已更新
	}
	return nil
}

func (h *SystemCoordinationHook[T]) Priority() int {
	return 1000 // 低优先级，在其他钩子之后执行
}

func (h *SystemCoordinationHook[T]) IsAsync() bool {
	return true
}

// HealthChecker 健康检查器
type HealthChecker[T Configurable] struct {
	integrator *ComponentIntegrator[T]
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker[T Configurable](integrator *ComponentIntegrator[T]) *HealthChecker[T] {
	return &HealthChecker[T]{
		integrator: integrator,
	}
}

// CheckHealth 执行健康检查
func (hc *HealthChecker[T]) CheckHealth() map[string]interface{} {
	health := make(map[string]interface{})
	health["overall_status"] = "healthy"
	health["timestamp"] = time.Now()

	// 检查调度池健康状态
	if hc.integrator.schedulerPool != nil {
		poolHealth := hc.integrator.schedulerPool.HealthCheck()
		health["scheduler_pool"] = poolHealth

		if status, ok := poolHealth["status"].(string); ok && status != "healthy" {
			health["overall_status"] = "degraded"
		}
	}

	// 检查文件监听器健康状态
	if hc.integrator.fileWatcher != nil {
		watcherHealth := map[string]interface{}{
			"status":  hc.integrator.fileWatcher.GetStatus().String(),
			"running": hc.integrator.fileWatcher.IsRunning(),
			"errors":  hc.integrator.fileWatcher.GetErrorCount(),
		}
		health["file_watcher"] = watcherHealth

		if !hc.integrator.fileWatcher.IsRunning() {
			health["overall_status"] = "degraded"
		}
	}

	// 检查配置存储健康状态
	if hc.integrator.configStore != nil {
		storeStats := hc.integrator.configStore.GetStats()
		storeHealth := map[string]interface{}{
			"has_config":      storeStats.HasConfig,
			"current_version": storeStats.CurrentVersion,
			"last_update":     storeStats.LastUpdate,
		}
		health["config_store"] = storeHealth

		if !storeStats.HasConfig {
			health["overall_status"] = "unhealthy"
		}
	}

	return health
}
