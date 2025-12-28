package config

import (
	"sync"
	"time"
)

// HookType 钩子类型枚举
//
// HookType 定义了配置管理器生命周期中可以注册钩子的各个阶段。
// 每个阶段都有特定的语义和执行时机，允许用户在适当的时候
// 注入自定义逻辑。
//
// 钩子类型说明：
// - HookTypeInit: 管理器初始化时触发
// - HookTypeBeforeLoad: 配置加载前触发
// - HookTypeAfterLoad: 配置加载后触发
// - HookTypeBeforeUpdate: 配置更新前触发
// - HookTypeAfterUpdate: 配置更新后触发
// - HookTypeError: 发生错误时触发
// - HookTypeDebug: 调试信息时触发
// - HookTypeInfo: 一般信息时触发
// - HookTypeWarn: 警告信息时触发
type HookType int

const (
	HookTypeInit HookType = iota
	HookTypeBeforeLoad
	HookTypeAfterLoad
	HookTypeBeforeUpdate
	HookTypeAfterUpdate
	HookTypeError
	HookTypeDebug
	HookTypeInfo
	HookTypeWarn
)

// HookContext 钩子执行上下文
//
// HookContext 包含了钩子执行时的所有上下文信息，为钩子处理器
// 提供了丰富的执行环境数据。
//
// 字段说明：
// - Type: 钩子类型，标识当前是哪种类型的钩子事件
// - Message: 描述性消息，说明当前事件的具体内容
// - Config: 相关的配置对象，可能是当前配置或新配置
// - Error: 错误信息（仅在错误类型钩子中有值）
// - Metadata: 元数据映射，可以包含任意的附加信息
// - Timestamp: 事件发生的时间戳
//
// 使用示例：
//
//	func myHook(ctx *HookContext) error {
//	    log.Printf("[%s] %s at %v", ctx.Type, ctx.Message, ctx.Timestamp)
//	    if ctx.Config != nil {
//	        // 处理配置相关逻辑
//	    }
//	    return nil
//	}
type HookContext struct {
	Type      HookType               // 钩子类型
	Message   string                 // 消息内容
	Config    interface{}            // 配置对象
	Error     error                  // 错误信息（如果有）
	Metadata  map[string]interface{} // 元数据
	Timestamp time.Time              // 时间戳
}

// HookHandler 钩子处理器接口
//
// HookHandler 定义了钩子处理器必须实现的接口。每个钩子处理器
// 都需要实现这三个方法来定义其行为特征。
//
// 接口方法：
// - Handle: 处理钩子事件的核心逻辑
// - Priority: 返回处理器的优先级（数值越小优先级越高）
// - IsAsync: 返回是否应该异步执行此处理器
//
// 实现示例：
//
//	type MyHookHandler struct {
//	    priority int
//	    async    bool
//	}
//
//	func (h *MyHookHandler) Handle(ctx *HookContext) error {
//	    // 处理逻辑
//	    return nil
//	}
//
//	func (h *MyHookHandler) Priority() int { return h.priority }
//	func (h *MyHookHandler) IsAsync() bool { return h.async }
type HookHandler interface {
	// Handle 处理钩子事件
	Handle(ctx *HookContext) error
	// Priority 返回处理器优先级，数值越小优先级越高
	Priority() int
	// IsAsync 返回是否异步执行
	IsAsync() bool
}

// HookRegistry 钩子注册表
//
// HookRegistry 是线程安全的钩子管理器，负责钩子处理器的注册、
// 注销和查询。它使用读写锁来保护并发访问，确保在多goroutine
// 环境下的安全性。
//
// 主要功能：
// - 注册钩子处理器到指定类型
// - 注销已注册的钩子处理器
// - 查询指定类型的所有处理器
// - 按优先级自动排序处理器
// - 提供统计和管理功能
//
// 线程安全性：
// 所有公共方法都是线程安全的，可以在多个goroutine中并发调用。
type HookRegistry struct {
	handlers map[HookType][]HookHandler // 按类型存储处理器
	mutex    sync.RWMutex               // 读写锁保护并发访问
}

// NewHookRegistry 创建新的钩子注册表
//
// 创建并初始化一个新的HookRegistry实例。返回的注册表
// 已经完全初始化，可以立即使用。
//
// 返回值：
//
//	*HookRegistry: 新创建的钩子注册表实例
//
// 使用示例：
//
//	registry := NewHookRegistry()
//	err := registry.RegisterHook(HookTypeInit, myHandler)
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		handlers: make(map[HookType][]HookHandler),
	}
}

// RegisterHook 注册钩子处理器
//
// 将钩子处理器注册到指定的钩子类型。如果处理器已经存在，
// 会返回错误。注册成功后，处理器会按照优先级自动排序。
//
// 参数：
//
//	hookType: 要注册的钩子类型
//	handler: 钩子处理器实例，不能为nil
//
// 返回值：
//
//	error: 注册失败时的错误信息，成功则返回nil
//
// 可能的错误：
// - ErrInvalidHandler: 处理器为nil
// - ErrHandlerAlreadyRegistered: 处理器已经注册
//
// 注意：同一个处理器实例只能注册一次到同一个钩子类型。
func (r *HookRegistry) RegisterHook(hookType HookType, handler HookHandler) error {
	if handler == nil {
		return ErrInvalidHandler
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 获取当前类型的处理器列表
	handlers := r.handlers[hookType]

	// 检查是否已经注册了相同的处理器
	for _, h := range handlers {
		if h == handler {
			return ErrHandlerAlreadyRegistered
		}
	}

	// 添加新处理器并按优先级排序
	handlers = append(handlers, handler)
	r.sortHandlersByPriority(handlers)
	r.handlers[hookType] = handlers

	return nil
}

// UnregisterHook 注销钩子处理器
func (r *HookRegistry) UnregisterHook(hookType HookType, handler HookHandler) error {
	if handler == nil {
		return ErrInvalidHandler
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	handlers := r.handlers[hookType]
	for i, h := range handlers {
		if h == handler {
			// 移除找到的处理器
			r.handlers[hookType] = append(handlers[:i], handlers[i+1:]...)
			return nil
		}
	}

	return ErrHandlerNotFound
}

// GetHandlers 获取指定类型的所有处理器
func (r *HookRegistry) GetHandlers(hookType HookType) []HookHandler {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	handlers := r.handlers[hookType]
	if len(handlers) == 0 {
		return nil
	}

	// 返回副本以避免外部修改
	result := make([]HookHandler, len(handlers))
	copy(result, handlers)
	return result
}

// GetAllHandlers 获取所有类型的处理器
func (r *HookRegistry) GetAllHandlers() map[HookType][]HookHandler {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[HookType][]HookHandler)
	for hookType, handlers := range r.handlers {
		if len(handlers) > 0 {
			handlersCopy := make([]HookHandler, len(handlers))
			copy(handlersCopy, handlers)
			result[hookType] = handlersCopy
		}
	}
	return result
}

// Clear 清空指定类型的所有处理器
func (r *HookRegistry) Clear(hookType HookType) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.handlers, hookType)
}

// ClearAll 清空所有处理器
func (r *HookRegistry) ClearAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.handlers = make(map[HookType][]HookHandler)
}

// Count 返回指定类型的处理器数量
func (r *HookRegistry) Count(hookType HookType) int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.handlers[hookType])
}

// TotalCount 返回所有处理器的总数量
func (r *HookRegistry) TotalCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	total := 0
	for _, handlers := range r.handlers {
		total += len(handlers)
	}
	return total
}

// sortHandlersByPriority 按优先级排序处理器（优先级数值越小越优先）
func (r *HookRegistry) sortHandlersByPriority(handlers []HookHandler) {
	for i := 0; i < len(handlers)-1; i++ {
		for j := i + 1; j < len(handlers); j++ {
			if handlers[i].Priority() > handlers[j].Priority() {
				handlers[i], handlers[j] = handlers[j], handlers[i]
			}
		}
	}
}

// 保持向后兼容的旧类型定义
type HookPattern = HookType

const (
	InitHook HookPattern = HookTypeInit
	Debug    HookPattern = HookTypeDebug
	Info     HookPattern = HookTypeInfo
	Warn     HookPattern = HookTypeWarn
	Error    HookPattern = HookTypeError
)

type HookHandlerFunc func(ctx HookContext)

func (h HookHandlerFunc) Exec(ctx HookContext) {
	if h == nil {
		return
	}
	h(ctx)
}

type Hook struct {
	Handles [9]HookHandlerFunc // 更新数组大小以匹配新的枚举数量
}

func NewHook() *Hook {
	return &Hook{}
}
