package vivid

// Behavior 定义 Actor 的消息处理核心逻辑函数类型。
// 每当 ActorContext 接收到一条消息时，当前 Behavior 会被调用，并传入该上下文。
// Behavior 实例应实现具体的消息处理、状态转移及行为切换等自定义逻辑。
type Behavior = func(ctx ActorContext)

// BehaviorOption 是用于配置 BehaviorOptions 的函数类型。
// 通常用于链式组合多个行为选项，实现灵活自定义。
type BehaviorOption = func(options *BehaviorOptions)

// BehaviorOptions 封装行为切换时可用的全部配置项。
// 主要包括行为堆栈的处理策略等。
type BehaviorOptions struct {
	// DiscardOld 指定行为切换时是否丢弃旧的行为。
	//   - true  : 新行为替换并抛弃堆栈中原有的行为（重置行为堆栈，仅保留新行为）。
	//   - false : 新行为压入堆栈顶，后续可通过 UnBecome 恢复先前行为。
	DiscardOld bool // 是否丢弃旧的行为
}

// WithBehaviorOptions 构造通用的 BehaviorOption，可整体设置 BehaviorOptions 对象。
// 适合一次性应用完整预置的行为配置项。
//
// 参数:
//   - options: 待生效的 BehaviorOptions 配置对象指针
// 返回值:
//   - BehaviorOption: 用于链式传递的行为选项配置函数
func WithBehaviorOptions(options *BehaviorOptions) BehaviorOption {
	return func(opts *BehaviorOptions) {
		*opts = *options
	}
}

// WithBehaviorDiscardOld 构造 BehaviorOption，用于指定切换行为时是否丢弃旧行为。
// 调用后生效于下一次行为变更。
//
// 参数:
//   - discardOld: true 表示切换时清空历史行为，仅保留新行为；false 为默认行为堆栈压栈模式，默认为 true。
// 返回值:
//   - BehaviorOption: 可链式配置的行为选项
func WithBehaviorDiscardOld(discardOld bool) BehaviorOption {
	return func(opts *BehaviorOptions) {
		opts.DiscardOld = discardOld
	}
}
