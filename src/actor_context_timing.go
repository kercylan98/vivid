package vivid

import (
	"github.com/kercylan98/chrono/timing"
	"time"
)

var (
	_ actorContextTimingInternal = (*actorContextTimingImpl)(nil)
)

func newActorContextTimingImpl(ctx ActorContext) *actorContextTimingImpl {
	impl := &actorContextTimingImpl{
		ActorContext: ctx,
	}

	if actorTimingWheel := ctx.getConfig().FetchTimingWheel(); actorTimingWheel != nil {
		impl.timing = actorTimingWheel.Named(ctx.Ref().GetPath())
	} else {
		impl.timing = ctx.System().getTimingWheel().Named(ctx.Ref().GetPath())
	}
	
	return impl
}

type actorContextTimingImpl struct {
	ActorContext
	timing timing.Named // 定时器
}

func (ctx *actorContextTimingImpl) getTimingWheel() timing.Named {
	return ctx.timing
}

func (ctx *actorContextTimingImpl) accidentAfter(name string, duration time.Duration, task accidentTimingTask) {
	ctx.timing.After(name, duration, timing.TaskFn(func() {
		ctx.tell(ctx.Ref(), task, SystemMessage)
	}))
}

func (ctx *actorContextTimingImpl) After(name string, duration time.Duration, task TimingTask) {
	ctx.timing.After(name, duration, timing.TaskFn(func() {
		ctx.tell(ctx.Ref(), task, UserMessage)
	}))
}

func (ctx *actorContextTimingImpl) ForeverLoop(name string, duration, interval time.Duration, task TimingTask) {
	ctx.timing.Loop(name, duration, timing.NewForeverLoopTask(interval, timing.TaskFn(func() {
		ctx.tell(ctx.Ref(), task, UserMessage)
	})))
}

func (ctx *actorContextTimingImpl) Loop(name string, duration, interval time.Duration, times int, task TimingTask) {
	ctx.timing.Loop(name, duration, timing.NewLoopTask(interval, times, timing.TaskFn(func() {
		ctx.tell(ctx.Ref(), task, UserMessage)
	})))
}

func (ctx *actorContextTimingImpl) Cron(name string, cron string, task TimingTask) error {
	return ctx.timing.Cron(name, cron, timing.TaskFn(func() {
		ctx.tell(ctx.Ref(), task, UserMessage)
	}))
}

func (ctx *actorContextTimingImpl) StopTimingTask(name string) {
	ctx.timing.Stop(name)
}

func (ctx *actorContextTimingImpl) ClearTimingTasks() {
	ctx.timing.Clear()
}
