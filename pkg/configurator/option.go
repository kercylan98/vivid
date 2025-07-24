package configurator

// Option 定义了泛型选项函数类型。
//
// Option 是选项模式的核心类型，用于配置对象的属性。
// 它是一个函数类型，接受配置对象并对其进行修改。
//
// 类型参数 C 是要配置的对象类型。
//
// 典型用法：
//   option := Option[*MyConfig](func(c *MyConfig) {
//       c.Field = "value"
//   })
//   option(myConfig)
type Option[C any] func(C)

// With 将当前选项与另一个配置函数组合。
//
// 此方法允许链式组合多个配置操作，先执行参数函数，再执行当前选项。
// 这提供了灵活的配置组合能力。
//
// 参数 f 是要组合的配置函数。
// 返回一个新的选项，包含组合后的配置逻辑。
//
// 示例：
//   option1 := Option[*Config](func(c *Config) { c.A = 1 })
//   option2 := option1.With(func(c *Config) { c.B = 2 })
//   // option2 会先设置 B=2，再设置 A=1
func (o Option[C]) With(f func(C)) Option[C] {
	return func(c C) {
		f(c)
		o(c)
	}
}
