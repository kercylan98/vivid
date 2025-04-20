package actx

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"time"
)

var _ actor.TimingContext = (*Timing)(nil)

func NewTiming(ctx actor.Context) *Timing {
	t := &Timing{
		ctx: ctx,
	}

	meta := ctx.MetadataContext()
	if tw := meta.Config().TimingWheel; tw == nil {
		t.timing = meta.System().GetTimingWheel().Named(meta.Ref().Path())
	} else {
		t.timing = tw.Named(meta.Ref().Path())
	}

	return t
}

type Timing struct {
	ctx    actor.Context
	timing timing.Named
}

func (t *Timing) After(name string, duration time.Duration, task timing.Task) {
	t.timing.After(name, duration, timing.TaskFN(func() {
		t.ctx.TransportContext().Tell(t.ctx.MetadataContext().Ref(), UserMessage, task)
	}))
}

func (t *Timing) Loop(name string, duration, interval time.Duration, times int, task timing.Task) {
	t.timing.Loop(name, duration, timing.NewLoopTask(interval, times, timing.TaskFN(func() {
		t.ctx.TransportContext().Tell(t.ctx.MetadataContext().Ref(), UserMessage, task)
	})))
}

func (t *Timing) ForeverLoop(name string, duration, interval time.Duration, task timing.Task) {
	t.timing.Loop(name, duration, timing.NewForeverLoopTask(interval, timing.TaskFN(func() {
		t.ctx.TransportContext().Tell(t.ctx.MetadataContext().Ref(), UserMessage, task)
	})))
}

func (t *Timing) Cron(name string, cron string, task timing.Task) error {
	return t.timing.Cron(name, cron, timing.TaskFN(func() {
		t.ctx.TransportContext().Tell(t.ctx.MetadataContext().Ref(), UserMessage, task)
	}))
}

func (t *Timing) Stop(name string) {
	t.timing.Stop(name)
}

func (t *Timing) Clear() {
	t.timing.Clear()
}
