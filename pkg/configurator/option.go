package configurator

// Option 定义了泛型选项函数类型。
//
// 选项模式的核心类型，用于封装单个配置操作。
// 支持组合和链式调用，提供灵活的配置方式。
type Option[C any] func(C)

// With 将当前选项与另一个配置函数组合。
//
// 先执行参数函数，再执行当前选项。
// 返回一个新的选项，支持链式调用。
func (o Option[C]) With(f func(C)) Option[C] {
    return func(c C) {
        f(c)
        o(c)
    }
}

// Apply 将选项应用到配置对象上。
//
// 提供与直接调用选项函数等价的语义化方法。
func (o Option[C]) Apply(configuration C) {
    o(configuration)
}
