package vivid

// TimingTask 是可以被 Actor 执行的定时任务，它是一个接口类型，允许创建有状态的定时任务
//   - 如果仅需要简单的执行函数，可以使用 TimingTaskFn 类型
type TimingTask interface {

	// Execute 执行定时任务
	Execute(ctx ActorContext)
}

// TimingTaskFn 是一个简单的定时任务函数类型，它可以被 Actor 执行
type TimingTaskFn func(ctx ActorContext)

func (f TimingTaskFn) Execute(ctx ActorContext) {
	f(ctx)
}

// 事故使用的定时任务，将会以系统消息处理
type accidentTimingTask func(ctx ActorContext)
