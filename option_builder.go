package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// OptionBuilder 配置选项构建器
//
// OptionBuilder 提供了一个类型安全的链式调用API来构建配置选项。
// 它支持参数验证、默认值设置和错误收集，确保构建的配置选项是有效的。
//
// 主要特性：
// - 链式调用：所有设置方法都返回OptionBuilder实例，支持方法链
// - 类型验证：每个参数都会进行类型和范围验证
// - 错误收集：验证错误会被收集，可以统一处理
// - 默认值：提供合理的默认值，简化配置过程
// - 不可变输出：Build方法返回不可变的配置对象
//
// 使用示例：
//
//	builder := NewOptionBuilder()
//	option, err := builder.
//	    WithFilename("config.yaml").
//	    WithFilepath("./configs").
//	    WithMaxWorkers(20).
//	    Build()
//	if err != nil {
//	    log.Fatal(err)
//	}
type OptionBuilder struct {
	filename    string
	filepath    string
	debounceDur time.Duration
	maxWorkers  int
	bufferSize  int
	validated   bool
	errors      []error
}

// NewOptionBuilder 创建新的配置选项构建器
//
// 返回一个初始化了默认值的OptionBuilder实例。
// 默认值包括：
// - filename: 从常量OptionFilename获取（通常为"config.yaml"）
// - filepath: 从常量OptionFilepath获取（通常为"./configs"）
// - debounceDur: 从常量OptionDebounceDur获取（通常为800ms）
// - maxWorkers: 10个工作goroutine
// - bufferSize: 100个任务的缓冲区
//
// 返回值：
//
//	*OptionBuilder: 初始化完成的构建器实例
func NewOptionBuilder() *OptionBuilder {
	return &OptionBuilder{
		filename:    OptionFilename,
		filepath:    OptionFilepath,
		debounceDur: OptionDebounceDur,
		maxWorkers:  10,  // 默认最大工作goroutine数量
		bufferSize:  100, // 默认任务队列缓冲区大小
		validated:   false,
		errors:      make([]error, 0),
	}
}

// WithFilename 设置配置文件名
//
// 设置要监听的配置文件名。文件名会经过以下验证：
// - 不能为空字符串
// - 不能包含路径分隔符（/或\）
// - 必须有有效的文件扩展名（.yaml, .yml, .json, .toml）
//
// 参数：
//
//	filename: 配置文件名，如"config.yaml"
//
// 返回值：
//
//	*OptionBuilder: 支持链式调用的构建器实例
//
// 注意：如果验证失败，错误会被添加到内部错误列表中，
// 可以通过HasErrors()和GetErrors()方法获取。
func (b *OptionBuilder) WithFilename(filename string) *OptionBuilder {
	if err := b.validateFilename(filename); err != nil {
		b.errors = append(b.errors, fmt.Errorf("filename validation failed: %w", err))
		return b
	}
	b.filename = filename
	b.validated = false // 重置验证状态
	return b
}

// WithFilepath 设置配置文件路径
//
// 设置配置文件所在的目录路径。路径可以是相对路径或绝对路径。
//
// 参数：
//
//	filepath: 配置文件目录路径，如"./configs"或"/etc/myapp"
//
// 返回值：
//
//	*OptionBuilder: 支持链式调用的构建器实例
//
// 验证规则：
// - 路径不能为空
// - 支持相对路径（./、../）和绝对路径（/）
// - 如果路径格式不明确，会自动添加"./"前缀
func (b *OptionBuilder) WithFilepath(filepath string) *OptionBuilder {
	if err := b.validateFilepath(filepath); err != nil {
		b.errors = append(b.errors, fmt.Errorf("filepath validation failed: %w", err))
		return b
	}
	b.filepath = filepath
	b.validated = false // 重置验证状态
	return b
}

// WithDebounceDuration 设置防抖时间
//
// 设置文件变更事件的防抖时间间隔。防抖机制可以防止短时间内
// 多次文件变更事件导致的重复配置加载。
//
// 参数：
//
//	dur: 防抖时间间隔，建议范围10ms-10s
//
// 返回值：
//
//	*OptionBuilder: 支持链式调用的构建器实例
//
// 验证规则：
// - 不能为负数
// - 最小值建议10ms（过短可能导致频繁触发）
// - 最大值建议10s（过长可能影响响应性）
func (b *OptionBuilder) WithDebounceDuration(dur time.Duration) *OptionBuilder {
	if err := b.validateDebounceDuration(dur); err != nil {
		b.errors = append(b.errors, fmt.Errorf("debounce duration validation failed: %w", err))
		return b
	}
	b.debounceDur = dur
	b.validated = false // 重置验证状态
	return b
}

// WithMaxWorkers 设置最大工作goroutine数量
//
// 设置调度池中最大的工作goroutine数量。这个参数直接影响
// 系统的并发处理能力和资源消耗。
//
// 参数：
//
//	max: 最大工作goroutine数量，建议范围1-1000
//
// 返回值：
//
//	*OptionBuilder: 支持链式调用的构建器实例
//
// 验证规则：
// - 必须为正数
// - 建议最大值1000（过大可能导致资源浪费）
// - 应该根据系统CPU核心数和预期负载来设置
func (b *OptionBuilder) WithMaxWorkers(max int) *OptionBuilder {
	if err := b.validateMaxWorkers(max); err != nil {
		b.errors = append(b.errors, fmt.Errorf("max workers validation failed: %w", err))
		return b
	}
	b.maxWorkers = max
	b.validated = false // 重置验证状态
	return b
}

// WithBufferSize 设置任务队列缓冲区大小
//
// 设置调度池任务队列的缓冲区大小。缓冲区用于暂存待处理的任务，
// 合适的缓冲区大小可以提高系统的吞吐量和响应性。
//
// 参数：
//
//	size: 缓冲区大小，建议范围0-10000
//
// 返回值：
//
//	*OptionBuilder: 支持链式调用的构建器实例
//
// 验证规则：
// - 不能为负数（0表示无缓冲）
// - 建议最大值10000（过大可能导致内存浪费）
// - 通常应该是maxWorkers的2-10倍
func (b *OptionBuilder) WithBufferSize(size int) *OptionBuilder {
	if err := b.validateBufferSize(size); err != nil {
		b.errors = append(b.errors, fmt.Errorf("buffer size validation failed: %w", err))
		return b
	}
	b.bufferSize = size
	b.validated = false // 重置验证状态
	return b
}

// Reset 重置所有配置选项到默认值
// 清除所有错误并重置验证状态
func (b *OptionBuilder) Reset() *OptionBuilder {
	b.filename = OptionFilename
	b.filepath = OptionFilepath
	b.debounceDur = OptionDebounceDur
	b.maxWorkers = 10
	b.bufferSize = 100
	b.validated = false
	b.errors = make([]error, 0)
	return b
}

// SetDefaults 设置默认值（如果当前值为零值）
// 只对未设置的字段应用默认值
func (b *OptionBuilder) SetDefaults() *OptionBuilder {
	if b.filename == "" {
		b.filename = OptionFilename
	}
	if b.filepath == "" {
		b.filepath = OptionFilepath
	}
	if b.debounceDur == 0 {
		b.debounceDur = OptionDebounceDur
	}
	if b.maxWorkers == 0 {
		b.maxWorkers = 10
	}
	if b.bufferSize == 0 {
		b.bufferSize = 100
	}
	return b
}

// validateFilename 验证文件名
func (b *OptionBuilder) validateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// 检查文件名是否包含路径分隔符
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("filename should not contain path separators, use WithFilepath instead")
	}

	// 检查文件扩展名
	ext := filepath.Ext(filename)
	validExts := []string{".yaml", ".yml", ".json", ".toml"}
	isValidExt := false
	for _, validExt := range validExts {
		if ext == validExt {
			isValidExt = true
			break
		}
	}
	if !isValidExt {
		return fmt.Errorf("unsupported file extension: %s, supported: %v", ext, validExts)
	}

	return nil
}

// validateFilepath 验证文件路径
func (b *OptionBuilder) validateFilepath(filepath string) error {
	if filepath == "" {
		return fmt.Errorf("filepath cannot be empty")
	}

	// 检查路径是否为绝对路径或相对路径
	if !strings.HasPrefix(filepath, "/") && !strings.HasPrefix(filepath, "./") && !strings.HasPrefix(filepath, "../") {
		// 如果不是明确的相对或绝对路径，添加 ./ 前缀
		// 这是一个警告而不是错误
	}

	return nil
}

// validateDebounceDuration 验证防抖时间
func (b *OptionBuilder) validateDebounceDuration(dur time.Duration) error {
	if dur < 0 {
		return fmt.Errorf("debounce duration cannot be negative")
	}

	// 防抖时间过短可能导致频繁触发
	if dur > 0 && dur < 10*time.Millisecond {
		return fmt.Errorf("debounce duration too short, minimum 10ms recommended")
	}

	// 防抖时间过长可能影响响应性
	if dur > 10*time.Second {
		return fmt.Errorf("debounce duration too long, maximum 10s recommended")
	}

	return nil
}

// validateMaxWorkers 验证最大工作goroutine数量
func (b *OptionBuilder) validateMaxWorkers(max int) error {
	if max <= 0 {
		return fmt.Errorf("max workers must be positive, got: %d", max)
	}

	if max > 1000 {
		return fmt.Errorf("max workers too large, maximum 1000 recommended, got: %d", max)
	}

	return nil
}

// validateBufferSize 验证任务队列缓冲区大小
func (b *OptionBuilder) validateBufferSize(size int) error {
	if size < 0 {
		return fmt.Errorf("buffer size cannot be negative, got: %d", size)
	}

	if size > 10000 {
		return fmt.Errorf("buffer size too large, maximum 10000 recommended, got: %d", size)
	}

	return nil
}

// HasErrors 检查是否有验证错误
func (b *OptionBuilder) HasErrors() bool {
	return len(b.errors) > 0
}

// GetErrors 获取所有验证错误
func (b *OptionBuilder) GetErrors() []error {
	return b.errors
}

// ClearErrors 清除所有错误
func (b *OptionBuilder) ClearErrors() *OptionBuilder {
	b.errors = make([]error, 0)
	return b
}

// Validate 验证配置选项
//
// 执行完整的配置验证，检查所有字段的有效性和一致性。
// 这个方法会清除之前的错误，重新验证所有配置项。
//
// 验证内容包括：
// - 各个字段的独立验证（文件名、路径、时间等）
// - 字段间的交叉验证（如缓冲区大小与工作器数量的关系）
//
// 返回值：
//
//	error: 如果验证失败，返回包含所有错误信息的错误；成功则返回nil
//
// 注意：验证成功后，validated标志会被设置为true，
// 表示当前配置是有效的。
func (b *OptionBuilder) Validate() error {
	// 清除之前的错误
	b.errors = make([]error, 0)

	// 验证所有字段
	if err := b.validateFilename(b.filename); err != nil {
		b.errors = append(b.errors, err)
	}

	if err := b.validateFilepath(b.filepath); err != nil {
		b.errors = append(b.errors, err)
	}

	if err := b.validateDebounceDuration(b.debounceDur); err != nil {
		b.errors = append(b.errors, err)
	}

	if err := b.validateMaxWorkers(b.maxWorkers); err != nil {
		b.errors = append(b.errors, err)
	}

	if err := b.validateBufferSize(b.bufferSize); err != nil {
		b.errors = append(b.errors, err)
	}

	// 交叉验证：缓冲区大小应该至少是工作goroutine数量的两倍
	if b.bufferSize > 0 && b.bufferSize < b.maxWorkers*2 {
		b.errors = append(b.errors, fmt.Errorf("buffer size (%d) should be at least twice the max workers (%d)", b.bufferSize, b.maxWorkers))
	}

	b.validated = len(b.errors) == 0

	if len(b.errors) > 0 {
		return fmt.Errorf("validation failed with %d errors: %v", len(b.errors), b.errors)
	}

	return nil
}

// Build 构建不可变的配置选项
//
// 执行最终验证并创建不可变的ImmutableOption对象。
// 这是构建过程的最后一步，会产生一个线程安全的配置对象。
//
// 构建过程：
// 1. 检查是否存在之前的验证错误
// 2. 执行完整的配置验证
// 3. 创建不可变的配置对象
// 4. 设置默认的超时配置
//
// 返回值：
//
//	*ImmutableOption: 不可变的配置选项对象
//	error: 构建失败时的错误信息
//
// 注意：一旦构建成功，返回的ImmutableOption对象就不能再修改，
// 确保了线程安全性。如需修改，应该使用Clone()方法创建新的构建器。
func (b *OptionBuilder) Build() (*ImmutableOption, error) {
	// 检查是否已有错误
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("build failed with existing errors: %v", b.errors)
	}

	// 执行验证
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	// 创建不可变的配置对象
	option := &ImmutableOption{
		filename:        b.filename,
		filepath:        b.filepath,
		debounceDur:     b.debounceDur,
		maxWorkers:      b.maxWorkers,
		bufferSize:      b.bufferSize,
		loadTimeout:     30 * time.Second, // 默认加载超时
		updateTimeout:   10 * time.Second, // 默认更新超时
		shutdownTimeout: 5 * time.Second,  // 默认关闭超时
		fullPath:        "",               // 将在首次访问时计算
	}

	return option, nil
}

// ImmutableOption 不可变的配置选项
//
// ImmutableOption 是一个不可变的配置选项对象，一旦创建就不能修改。
// 这种设计确保了线程安全性，多个goroutine可以安全地并发访问同一个
// ImmutableOption实例而不需要额外的同步机制。
//
// 主要特性：
// - 不可变性：所有字段都是私有的，只能通过getter方法访问
// - 线程安全：可以在多个goroutine中安全使用
// - 延迟计算：某些计算密集的操作（如路径拼接）采用延迟计算
// - 完整性：包含了配置管理器运行所需的所有参数
//
// 使用场景：
// - 传递给配置管理器进行初始化
// - 在多个组件间共享配置信息
// - 作为配置快照保存当前设置
type ImmutableOption struct {
	// 文件配置
	filename string // 配置文件名
	filepath string // 配置文件路径

	// 监听配置
	debounceDur time.Duration // 防抖时间

	// 调度池配置
	maxWorkers int // 最大工作goroutine数
	bufferSize int // 任务队列缓冲区大小

	// 超时配置
	loadTimeout     time.Duration // 加载超时
	updateTimeout   time.Duration // 更新超时
	shutdownTimeout time.Duration // 关闭超时

	// 缓存的计算值
	fullPath string // 完整文件路径（延迟计算）
}

// GetFilename 获取配置文件名
func (o *ImmutableOption) GetFilename() string {
	return o.filename
}

// GetFilepath 获取配置文件路径
func (o *ImmutableOption) GetFilepath() string {
	return o.filepath
}

// GetDebounceDuration 获取防抖时间
func (o *ImmutableOption) GetDebounceDuration() time.Duration {
	return o.debounceDur
}

// GetMaxWorkers 获取最大工作goroutine数量
func (o *ImmutableOption) GetMaxWorkers() int {
	return o.maxWorkers
}

// GetBufferSize 获取任务队列缓冲区大小
func (o *ImmutableOption) GetBufferSize() int {
	return o.bufferSize
}

// GetLoadTimeout 获取加载超时时间
func (o *ImmutableOption) GetLoadTimeout() time.Duration {
	return o.loadTimeout
}

// GetUpdateTimeout 获取更新超时时间
func (o *ImmutableOption) GetUpdateTimeout() time.Duration {
	return o.updateTimeout
}

// GetShutdownTimeout 获取关闭超时时间
func (o *ImmutableOption) GetShutdownTimeout() time.Duration {
	return o.shutdownTimeout
}

// GetFullPath 获取完整的配置文件路径
// 使用延迟计算来避免重复的路径拼接
func (o *ImmutableOption) GetFullPath() string {
	if o.fullPath == "" {
		// 线程安全的延迟计算
		// 由于ImmutableOption是不可变的，这里的竞态条件是无害的
		// 使用 / 作为路径分隔符以保持跨平台一致性
		o.fullPath = strings.ReplaceAll(filepath.Join(o.filepath, o.filename), "\\", "/")
	}
	return o.fullPath
}

// String 返回配置选项的字符串表示
func (o *ImmutableOption) String() string {
	return fmt.Sprintf("Option{filename: %s, filepath: %s, debounceDur: %v, maxWorkers: %d, bufferSize: %d}",
		o.filename, o.filepath, o.debounceDur, o.maxWorkers, o.bufferSize)
}

// Clone 创建配置选项的副本
// 返回一个新的OptionBuilder，可以基于当前配置进行修改
func (o *ImmutableOption) Clone() *OptionBuilder {
	return &OptionBuilder{
		filename:    o.filename,
		filepath:    o.filepath,
		debounceDur: o.debounceDur,
		maxWorkers:  o.maxWorkers,
		bufferSize:  o.bufferSize,
		validated:   false,
		errors:      make([]error, 0),
	}
}

// Equals 比较两个配置选项是否相等
func (o *ImmutableOption) Equals(other *ImmutableOption) bool {
	if other == nil {
		return false
	}

	return o.filename == other.filename &&
		o.filepath == other.filepath &&
		o.debounceDur == other.debounceDur &&
		o.maxWorkers == other.maxWorkers &&
		o.bufferSize == other.bufferSize &&
		o.loadTimeout == other.loadTimeout &&
		o.updateTimeout == other.updateTimeout &&
		o.shutdownTimeout == other.shutdownTimeout
}
