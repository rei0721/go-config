package config

import "github.com/fsnotify/fsnotify"

type HandlerFunc func(ctx *Context)

type Context struct {
	Config  any // 全局配置对象
	FSEvent fsnotify.Event
}
