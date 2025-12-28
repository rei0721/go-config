package main

import (
	"fmt"
	"time"

	config "github.com/rei0721/go-config"
)

func main() {
	// 演示新的OptionBuilder API
	fmt.Println("=== Option Builder 示例 ===")

	// 1. 基本链式调用
	fmt.Println("\n1. 基本链式调用:")
	builder := config.NewOptionBuilder()
	option, err := builder.
		WithFilename("app.yaml").
		WithFilepath("./config").
		WithDebounceDuration(500 * time.Millisecond).
		WithMaxWorkers(8).
		WithBufferSize(200).
		Build()

	if err != nil {
		fmt.Printf("构建失败: %v\n", err)
		return
	}

	fmt.Printf("配置选项: %s\n", option.String())
	fmt.Printf("完整路径: %s\n", option.GetFullPath())

	// 2. 默认值演示
	fmt.Println("\n2. 默认值演示:")
	defaultOption, err := config.NewOptionBuilder().Build()
	if err != nil {
		fmt.Printf("构建失败: %v\n", err)
		return
	}
	fmt.Printf("默认配置: %s\n", defaultOption.String())

	// 3. 验证错误演示
	fmt.Println("\n3. 验证错误演示:")
	invalidBuilder := config.NewOptionBuilder()
	_, err = invalidBuilder.
		WithFilename(""). // 无效文件名
		Build()

	if err != nil {
		fmt.Printf("预期的验证错误: %v\n", err)
	}

	// 4. 重置和克隆演示
	fmt.Println("\n4. 重置和克隆演示:")
	builder.Reset()
	resetOption, _ := builder.Build()
	fmt.Printf("重置后的配置: %s\n", resetOption.String())

	clonedBuilder := option.Clone()
	clonedOption, _ := clonedBuilder.
		WithMaxWorkers(16).
		Build()
	fmt.Printf("克隆并修改后的配置: %s\n", clonedOption.String())

	// 5. 相等性比较
	fmt.Println("\n5. 相等性比较:")
	option1, _ := config.NewOptionBuilder().Build()
	option2, _ := config.NewOptionBuilder().Build()
	fmt.Printf("两个默认配置是否相等: %t\n", option1.Equals(option2))
	fmt.Printf("默认配置与自定义配置是否相等: %t\n", option1.Equals(option))
}
