package config

import (
	"fmt"
)

type TestIntegrationConfig struct {
	Name string `yaml:"name"`
}

func testIntegration() {
	fmt.Println("开始测试...")

	defaultConfig := &TestIntegrationConfig{Name: "test"}
	fmt.Println("默认配置创建完成")

	manager := CreateSimpleManager(defaultConfig)
	fmt.Println("管理器创建完成")

	fmt.Printf("管理器类型: %T\n", manager)
	fmt.Printf("重构模式: %t\n", manager.IsRefactoredMode())

	fmt.Println("测试完成")
}
