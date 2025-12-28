package config

import (
	"testing"
	"time"
)

// TestOptionBuilder_BasicChaining 测试基本的链式调用功能
func TestOptionBuilder_BasicChaining(t *testing.T) {
	builder := NewOptionBuilder()

	// 测试链式调用
	result := builder.
		WithFilename("test.yaml").
		WithFilepath("./test").
		WithDebounceDuration(500 * time.Millisecond).
		WithMaxWorkers(5).
		WithBufferSize(50)

	// 验证返回的是同一个实例
	if result != builder {
		t.Error("Chain methods should return the same builder instance")
	}

	// 验证值是否正确设置
	if builder.filename != "test.yaml" {
		t.Errorf("Expected filename 'test.yaml', got '%s'", builder.filename)
	}

	if builder.filepath != "./test" {
		t.Errorf("Expected filepath './test', got '%s'", builder.filepath)
	}

	if builder.debounceDur != 500*time.Millisecond {
		t.Errorf("Expected debounce duration 500ms, got %v", builder.debounceDur)
	}

	if builder.maxWorkers != 5 {
		t.Errorf("Expected max workers 5, got %d", builder.maxWorkers)
	}

	if builder.bufferSize != 50 {
		t.Errorf("Expected buffer size 50, got %d", builder.bufferSize)
	}
}

// TestOptionBuilder_Validation 测试输入验证功能
func TestOptionBuilder_Validation(t *testing.T) {
	builder := NewOptionBuilder()

	// 测试无效文件名
	builder.WithFilename("")
	if !builder.HasErrors() {
		t.Error("Expected validation error for empty filename")
	}

	// 清除错误并测试无效路径
	builder.ClearErrors().WithFilename("valid.yaml").WithFilepath("")
	if !builder.HasErrors() {
		t.Error("Expected validation error for empty filepath")
	}

	// 测试负数防抖时间
	builder.ClearErrors().WithFilepath("./valid").WithDebounceDuration(-1 * time.Second)
	if !builder.HasErrors() {
		t.Error("Expected validation error for negative debounce duration")
	}

	// 测试无效的最大工作数
	builder.ClearErrors().WithDebounceDuration(100 * time.Millisecond).WithMaxWorkers(0)
	if !builder.HasErrors() {
		t.Error("Expected validation error for zero max workers")
	}

	// 测试负数缓冲区大小
	builder.ClearErrors().WithMaxWorkers(5).WithBufferSize(-1)
	if !builder.HasErrors() {
		t.Error("Expected validation error for negative buffer size")
	}
}

// TestOptionBuilder_DefaultValues 测试默认值功能
func TestOptionBuilder_DefaultValues(t *testing.T) {
	builder := NewOptionBuilder()

	// 验证默认值
	if builder.filename != OptionFilename {
		t.Errorf("Expected default filename '%s', got '%s'", OptionFilename, builder.filename)
	}

	if builder.filepath != OptionFilepath {
		t.Errorf("Expected default filepath '%s', got '%s'", OptionFilepath, builder.filepath)
	}

	if builder.debounceDur != OptionDebounceDur {
		t.Errorf("Expected default debounce duration %v, got %v", OptionDebounceDur, builder.debounceDur)
	}

	// 测试重置功能
	builder.WithFilename("custom.yaml").WithMaxWorkers(20)
	builder.Reset()

	if builder.filename != OptionFilename {
		t.Errorf("Reset should restore default filename, got '%s'", builder.filename)
	}

	if builder.maxWorkers != 10 {
		t.Errorf("Reset should restore default max workers, got %d", builder.maxWorkers)
	}

	if builder.HasErrors() {
		t.Error("Reset should clear all errors")
	}
}

// TestImmutableOption_Build 测试构建不可变对象
func TestImmutableOption_Build(t *testing.T) {
	builder := NewOptionBuilder()

	// 构建有效的配置
	option, err := builder.
		WithFilename("config.yaml").
		WithFilepath("./configs").
		WithDebounceDuration(800 * time.Millisecond).
		WithMaxWorkers(10).
		WithBufferSize(100).
		Build()

	if err != nil {
		t.Fatalf("Build should succeed with valid configuration: %v", err)
	}

	if option == nil {
		t.Fatal("Build should return non-nil option")
	}

	// 验证不可变对象的值
	if option.GetFilename() != "config.yaml" {
		t.Errorf("Expected filename 'config.yaml', got '%s'", option.GetFilename())
	}

	if option.GetFilepath() != "./configs" {
		t.Errorf("Expected filepath './configs', got '%s'", option.GetFilepath())
	}

	if option.GetDebounceDuration() != 800*time.Millisecond {
		t.Errorf("Expected debounce duration 800ms, got %v", option.GetDebounceDuration())
	}

	// 测试完整路径计算
	expectedPath := "configs/config.yaml"
	if option.GetFullPath() != expectedPath {
		t.Errorf("Expected full path '%s', got '%s'", expectedPath, option.GetFullPath())
	}
}

// TestImmutableOption_BuildWithErrors 测试构建失败的情况
func TestImmutableOption_BuildWithErrors(t *testing.T) {
	builder := NewOptionBuilder()

	// 尝试构建无效配置
	option, err := builder.
		WithFilename(""). // 无效文件名
		Build()

	if err == nil {
		t.Error("Build should fail with invalid configuration")
	}

	if option != nil {
		t.Error("Build should return nil option on error")
	}
}

// TestImmutableOption_Immutability 测试不可变性
func TestImmutableOption_Immutability(t *testing.T) {
	builder := NewOptionBuilder()
	option, err := builder.Build()

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// 验证不可变对象没有公开的设置方法
	// 这是通过编译时检查来保证的，这里我们测试克隆功能
	clonedBuilder := option.Clone()

	if clonedBuilder == nil {
		t.Error("Clone should return non-nil builder")
	}

	// 修改克隆的构建器不应该影响原始选项
	clonedBuilder.WithFilename("modified.yaml")

	if option.GetFilename() == "modified.yaml" {
		t.Error("Modifying cloned builder should not affect original option")
	}
}

// TestImmutableOption_Equals 测试相等性比较
func TestImmutableOption_Equals(t *testing.T) {
	builder1 := NewOptionBuilder()
	option1, _ := builder1.Build()

	builder2 := NewOptionBuilder()
	option2, _ := builder2.Build()

	// 相同配置应该相等
	if !option1.Equals(option2) {
		t.Error("Options with same configuration should be equal")
	}

	// 不同配置应该不相等
	builder3 := NewOptionBuilder().WithMaxWorkers(20)
	option3, _ := builder3.Build()

	if option1.Equals(option3) {
		t.Error("Options with different configuration should not be equal")
	}

	// nil比较
	if option1.Equals(nil) {
		t.Error("Option should not equal nil")
	}
}
