# 配置管理库示例

本目录包含了配置管理库的各种使用示例，展示了从基础用法到高级功能的完整使用方式。

## 示例列表

### 1. basic - 基础示例
展示配置管理库的基本使用方法，包括：
- 创建配置管理器
- 加载和获取配置
- 更新配置
- 文件监听

```bash
cd basic
go run main.go
```

### 2. compatibility - 向后兼容性示例
演示新旧API的兼容性和迁移路径：
- 传统API使用方式
- 兼容性层使用
- 渐进式迁移
- 完全新API使用
- 兼容性检查

```bash
cd compatibility
go run main.go
```

### 3. integrated - 集成示例
展示完整的集成配置管理器功能：
- 使用工厂函数创建管理器
- 调度池功能
- 拦截统计
- 优雅关闭

```bash
cd integrated
go run main.go
```

注意：integrated目录还包含`simple_example.go`，这是一个简化版本的示例。

### 4. option_builder - 选项构建器示例
演示OptionBuilder的链式调用API：
- 基本链式调用
- 默认值设置
- 验证错误处理
- 重置和克隆
- 相等性比较

```bash
cd option_builder
go run main.go
```

### 5. scheduler_pool - 调度池示例
展示调度池的独立使用：
- 创建和启动调度池
- 提交任务
- 统计信息
- 健康检查

```bash
cd scheduler_pool
go run main.go
```

## 构建所有示例

每个示例目录都有独立的`go.mod`文件，可以单独构建：

```bash
# 构建所有示例
for dir in basic compatibility integrated option_builder scheduler_pool; do
    cd $dir
    go build -o example_${dir}.exe
    cd ..
done
```

## 配置文件

示例会在运行时自动创建必要的配置文件和目录。默认配置文件位置：
- `./configs/config.yaml` (basic, option_builder, scheduler_pool)
- `./configs/app_config.yaml` (compatibility)
- `./configs/simple_config.yaml` (integrated)

## 注意事项

1. 所有示例都使用相对路径，确保在对应的示例目录中运行
2. 示例会自动创建配置文件，首次运行时是正常的
3. 某些示例可能需要几秒钟来演示异步功能
4. 如果遇到端口冲突，可以修改示例中的端口配置

## 故障排除

如果遇到构建问题：

1. 确保Go版本 >= 1.23.0
2. 运行 `go mod tidy` 更新依赖
3. 检查是否在正确的目录中运行命令
4. 确保根目录的go.mod文件存在且正确

如果遇到运行时问题：

1. 检查文件权限，确保可以创建配置文件
2. 确保没有其他程序占用相同的配置文件
3. 查看控制台输出的错误信息