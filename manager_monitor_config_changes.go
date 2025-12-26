package config

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Unmarshal 解析配置到结构体
func (m *Manager[T]) Unmarshal() error {
	var newConfig T
	if err := m.vp.Unmarshal(&newConfig); err != nil {
		m.hooks.Handles[Error].Exec(HookContext{
			Message: fmt.Sprintf("failed to unmarshal new config: %v", err),
		})
		return errors.New(fmt.Sprintf("failed to unmarshal new config: %v", err))
	}

	m.rwMutex.Lock()
	defer m.rwMutex.Unlock()

	oldConfig := *m.config
	changes := make(map[string][2]any)

	if !compareStructs(oldConfig, newConfig, "", changes) {
		m.hooks.Handles[Error].Exec(HookContext{
			Message: "config type mismatch, changes blocked",
		})
		return errors.New(fmt.Sprintf("config type mismatch, changes blocked"))
	}

	m.config = &newConfig
	return nil
}

// monitorConfigChanges 监听配置变更（带防抖与类型过滤）
func (m *Manager[T]) monitorConfigChanges(handles []HandlerFunc) {
	m.vp.WatchConfig()
	m.vp.OnConfigChange(func(e fsnotify.Event) {
		// 仅响应写入事件，忽略 CHMOD/RENAME 等
		if e.Op != fsnotify.Write {
			return
		}

		// 防抖处理：忽略短时间内的重复变更
		if time.Since(m.lastChange) < m.debounceDur {
			return
		}
		m.lastChange = time.Now()

		m.hooks.Handles[Info].Exec(HookContext{
			Message: fmt.Sprintf("[config] 检测到文件变更: %s", e.Name),
		})

		// 重新加载配置
		if err := m.vp.ReadInConfig(); err != nil {
			m.hooks.Handles[Info].Exec(HookContext{
				Message: fmt.Sprintf("[config] 重新加载失败: %v", err),
			})
			return
		}

		// 解析配置到结构体
		if err := m.Unmarshal(); err != nil {
			return
		}

		// 创建中间件上下文
		ctx := &Context{
			Config:  m.config,
			FSEvent: e,
		}

		// 执行中间件
		for _, handle := range handles {
			handle(ctx)
		}
	})
}

// compareStructs 比较结构体并收集变更
// 参数：
//
//	oldObj: 旧结构体
//	newObj: 新结构体
//	prefix: 字段路径前缀
//	changes: 记录变更的映射
//
// 返回值：
//
//	bool: 结构体类型是否一致
func compareStructs(oldObj, newObj any, prefix string, changes map[string][2]any) bool {
	oldVal := reflect.ValueOf(oldObj)
	newVal := reflect.ValueOf(newObj)

	if oldVal.Type() != newVal.Type() {
		return false
	}

	if oldVal.Kind() != reflect.Struct {
		return true
	}

	for i := 0; i < oldVal.NumField(); i++ {
		oldField := oldVal.Field(i)
		newField := newVal.Field(i)
		fieldName := oldVal.Type().Field(i).Name
		fullName := prefix + fieldName

		if oldField.Kind() == reflect.Struct {
			if !compareStructs(oldField.Interface(), newField.Interface(), fullName+".", changes) {
				return false
			}
			continue
		}

		if oldField.Kind() != newField.Kind() {
			return false
		}

		if !reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
			changes[fullName] = [2]any{oldField.Interface(), newField.Interface()}
		}
	}

	return true
}
