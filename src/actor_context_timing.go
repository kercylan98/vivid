package vivid

import (
	"github.com/kercylan98/chrono/timing"
	"time"
)

var (
	_ actorContextTimingInternal = (*actorContextTimingImpl)(nil)
)

func newActorContextTimingImpl(ctx ActorContext) *actorContextTimingImpl {
	return &actorContextTimingImpl{
		ActorContext: ctx,
		timing:       ctx.System().getTimingWheel().Named(ctx.Ref().GetPath()),
	}
}

type actorContextTimingImpl struct {
	ActorContext
	timing timing.Named // 定时器
}

func (ctx *actorContextTimingImpl) getTimingWheel() timing.Named {
	return ctx.timing
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
