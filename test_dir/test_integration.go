package main

import (
	"fmt"

	config "github.com/rei0721/go-config"
)

type TestConfig struct {
	Name string `yaml:"name"`
}

func main() {
	fmt.Println("开始测试...")

	defaultConfig := &TestConfig{Name: "test"}
	fmt.Println("默认配置创建完成")

	manager := config.CreateSimpleManager(defaultConfig)
	fmt.Println("管理器创建完成")

	fmt.Printf("管理器类型: %T\n", manager)
	fmt.Printf("重构模式: %t\n", manager.IsRefactoredMode())

	fmt.Println("测试完成")
}
