package config

import (
	"context"
	"fmt"
	"time"
)

// CreateSimpleManager 创建简单的配置管理器
// 使用默认配置，适合快速开始
func CreateSimpleManager[T Configurable](defaultConfig *T) *Manager[T] {
	return NewManager(defaultConfig).EnableRefactoredMode()
}

// CreateAdvancedManager 创建高级配置管理器
// 支持自定义选项和钩子
func CreateAdvancedManager[T Configurable](
	defaultConfig *T,
	option *ImmutableOption,
	hooks map[HookType]HookHandler,
) (*Manager[T], error) {
	builder := NewIntegratedManagerBuilder(defaultConfig).
		WithOption(option).
		WithRefactoredMode(true)

	// 添加钩子
	for hookType, handler := range hooks {
		builder.WithHook(hookType, handler)
	}

	ctx := context.Background()
	return builder.BuildAndInitialize(ctx)
}

// CreateProductionManager 创建生产环境配置管理器
// 包含完整的错误处理、监控和恢复机制
func CreateProductionManager[T Configurable](
	defaultConfig *T,
	configPath string,
	logger Logger,
) (*Manager[T], error) {
	// 构建生产环境选项
	option, err := NewOptionBuilder().
		WithFilepath("./configs").
		WithFilename("config.yaml").
		WithDebounceDuration(500 * time.Millisecond).
		WithMaxWorkers(20).
		WithBufferSize(200).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build production option: %w", err)
	}

	// 创建错误处理器
	errorHandler := NewDefaultErrorHandler(logger)

	// 创建生产环境钩子
	hooks := map[HookType]HookHandler{
		HookTypeError: &ProductionErrorHook{logger: logger},
		HookTypeInfo:  &ProductionInfoHook{logger: logger},
		HookTypeWarn:  &ProductionWarnHook{logger: logger},
	}

	builder := NewIntegratedManagerBuilder(defaultConfig).
		WithOption(option).
		WithRefactoredMode(true).
		WithErrorHandler(errorHandler)

	for hookType, handler := range hooks {
		builder.WithHook(hookType, handler)
	}

	ctx := context.Background()
	manager, err := builder.BuildAndInitialize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize production manager: %w", err)
	}

	return manager, nil
}

// CreateIntegratedManager 创建完全集成的配置管理器
// 这是推荐的创建方式，提供最佳的性能和功能
func CreateIntegratedManager[T Configurable](
	defaultConfig *T,
	configFilename string,
	configPath string,
) (*Manager[T], error) {
	// 构建选项
	option, err := NewOptionBuilder().
		WithFilename(configFilename).
		WithFilepath(configPath).
		WithDebounceDuration(300 * time.Millisecond).
		WithMaxWorkers(10).
		WithBufferSize(100).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build option: %w", err)
	}

	// 创建管理器
	manager := NewManager(defaultConfig).EnableRefactoredMode()

	// 初始化
	ctx := context.Background()
	if err := manager.InitWithOptions(ctx, option); err != nil {
		return nil, fmt.Errorf("failed to initialize manager: %w", err)
	}

	return manager, nil
}

// CreateManagerWithBuilder 使用构建器创建管理器
// 提供最大的灵活性和控制
func CreateManagerWithBuilder[T Configurable](
	defaultConfig *T,
	builderFunc func(*IntegratedManagerBuilder[T]) *IntegratedManagerBuilder[T],
) (*Manager[T], error) {
	builder := NewIntegratedManagerBuilder(defaultConfig)

	// 应用用户自定义配置
	if builderFunc != nil {
		builder = builderFunc(builder)
	}

	ctx := context.Background()
	return builder.BuildAndInitialize(ctx)
}
