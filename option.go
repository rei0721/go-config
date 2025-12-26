package config

import (
	"time"
)

type Option struct {
	value       string
	pathValue   string
	fileValue   string
	Filename    OptionString
	Filepath    OptionString
	DebounceDur OptionTimeDuration
}

// NewOption 创建默认配置
func NewOption() *Option {
	opt := &Option{}
	opt.setDefaultValue()
	return opt
}

// 初始化默认值
func (s *Option) setDefaultValue() *Option {
	s.Filename.Set(OptionFilename, false)
	s.Filepath.Set(OptionFilepath, false)
	s.DebounceDur.Set(OptionTimeDuration(OptionDebounceDur), false)
	return s
}

type OptionString string
type OptionTimeDuration time.Duration

func (o *OptionString) Set(newStr OptionString, reset ...bool) {
	if len(reset) == 0 {
		reset = []bool{true}
	}
	if *o != "" && !reset[0] {
		return
	}
	*o = newStr
}

func (o *OptionString) ToValue() string {
	return string(*o)
}

func (o *OptionTimeDuration) Set(newDate OptionTimeDuration, reset ...bool) {
	if len(reset) == 0 {
		reset = []bool{true}
	}
	if *o != 0 && !reset[0] {
		return
	}
	*o = newDate
}

func (o *OptionTimeDuration) ToValue() time.Duration {
	return time.Duration(*o)
}

func (s *Option) File() string {
	if s.fileValue != "" {
		return s.fileValue
	}
	s.fileValue = ConfigPath(s.Filepath.ToValue(), s.Filename.ToValue(), true)
	return s.File()
}

func (s *Option) Path() string {
	if s.pathValue != "" {
		return s.pathValue
	}
	s.pathValue = ConfigPath(s.Filepath.ToValue(), "", true)
	return s.Path()
}
