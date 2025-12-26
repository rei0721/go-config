package config

type HookPattern int

const (
	InitHook HookPattern = iota
	Debug
	Info
	Warn
	Error
	hookIndex
)

type HookContext struct {
	Message string
	Pattern HookPattern
}

type HookHandlerFunc func(ctx HookContext)

func (h HookHandlerFunc) Exec(ctx HookContext) {
	if h == nil {
		return
	}
	h(ctx)
}

type Hook struct {
	Handles [hookIndex]HookHandlerFunc
}

func NewHook() *Hook {
	return &Hook{}
}

//func (hooks *Hook) SetHook(index HookPattern, h HookHandlerFunc) *Hook {
//	SetHook(index, h)
//	return hooks
//}

func (m *Manager[T]) SetHook(index HookPattern, h HookHandlerFunc) *Manager[T] {
	hooks := m.hooks
	hooks.Handles[index] = h
	return m
}
