package config

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type Configurable interface {
	// 可以添加必要的方法约束
}

type ConfigManager[T Configurable] interface {
	Init(ctx context.Context, handles ...HandlerFunc) error
	GetConfig() (*T, error)
	UpdateField(ctx context.Context, updateFunc func(*T)) error
	SetHook(pattern HookPattern, handler HookHandlerFunc) ConfigManager[T]
}

type Manager[T Configurable] struct {
	config             *T            // 全局配置对象
	vp                 *viper.Viper  // Viper 实例
	rwMutex            sync.RWMutex  // 读写锁
	lastChange         time.Time     // 上次触发时间（用于防抖）
	debounceDur        time.Duration // 防抖间隔
	hooks              *Hook         // hook
	pathName           string        // 配置文件
	opts               *Option       // 设置选项
	optsInit           bool          // 初始化选项
	initializeValidate bool          // 初始化验证
	defaultConfig      *T            // default config
}

func NewManager[T Configurable](defaultConfig *T) *Manager[T] {
	return &Manager[T]{
		config:     defaultConfig,
		vp:         viper.New(),
		lastChange: time.Time{},
		// opts:       NewOption(),
		hooks: NewHook(),
	}
}
