package config

import (
	"fmt"
)

func (m *Manager[T]) SetOption(opts *Option) {
	if !m.optsInit {
		// 标记已初始化option
		m.optsInit = true
		// 初始化设置选项默认值
		if opts != nil {
			opts.setDefaultValue()
		} else {
			opts = NewOption()
		}
		m.opts = opts
	}
}

func (m *Manager[T]) Load(handles ...HandlerFunc) error {
	if m.initializeValidate {
		return nil
	}

	// 如果option不存在配置则设置默认选项
	m.SetOption(nil)

	// hook init
	m.hooks.Handles[InitHook].Exec(HookContext{
		Message: "开始初始化",
	})

	// setting debouncedur
	m.debounceDur = m.opts.DebounceDur.ToValue()

	inFile := m.opts.File()

	m.vp.SetConfigFile(inFile)

	// m.vp.SetConfigType(m.opts.FileType.ToValue()) // 设置文件类型
	// // 根据环境加载不同配置文件 设置文件名
	// if m.opts.Env != "" {
	// 	// 设置文件名.环境名
	// 	m.vp.SetConfigName(fmt.Sprintf("%s.%s", m.opts.Filename, m.opts.Env))
	// } else {
	// 	// 设置文件名
	// 	m.vp.SetConfigName(m.opts.Filename.ToValue())
	// }
	// m.vp.AddConfigPath(absPath) // 设置文件路径

	// 如果文件不存在，则创建默认配置文件
	if err := m.ensureConfigFile(m.opts); err != nil {
		m.hooks.Handles[Error].Exec(HookContext{
			Message: fmt.Sprintf("[config] 创建默认配置文件失败: %v", err),
		})
		return err
	}

	// 读取配置文件
	if err := m.vp.ReadInConfig(); err != nil {
		m.hooks.Handles[Error].Exec(HookContext{
			Message: fmt.Sprintf("[config] 加载配置失败: %v", err),
		})
		return err
	}

	m.hooks.Handles[Info].Exec(HookContext{
		Message: fmt.Sprintf("[config] 已加载配置文件: %s", m.vp.ConfigFileUsed()),
	})

	// 解析配置到结构体
	if err := m.Unmarshal(); err != nil {
		m.hooks.Handles[Error].Exec(HookContext{
			Message: fmt.Sprintf("[config] 解析配置到结构体失败 Error: %s", err.Error()),
		})
		return err
	}

	// 监听配置变更
	m.monitorConfigChanges(handles)

	// 验证配置通过
	m.initializeValidateSetting(true)

	return nil
}
