package vivid

import (
	"sync"
	"time"
)

var _ Actor = (*routerMonitorActor)(nil)
var metricsPool = sync.Pool{New: func() any {
	return &RouterMetrics{}
}}

func newRouterMonitorActor(actor Actor) *routerMonitorActor {
	return &routerMonitorActor{
		actor: actor,
	}
}

type routerMonitorActor struct {
	actor Actor
}

func (r *routerMonitorActor) Receive(context ActorContext) {
	switch context.Message().(type) {
	case *OnLaunch, *OnKill, OnKilled, OnLaunch, OnPreRestart, OnRestart, OnWatchEnd:
		r.actor.Receive(context)
	default:
		startAt := time.Now()
		defer func() {
			m := metricsPool.Get().(*RouterMetrics)
			r.onMonitor(context, startAt, m)
		}()
		r.actor.Receive(context)
	}
}

func (r *routerMonitorActor) onMonitor(context ActorContext, startAt time.Time, m *RouterMetrics) {
	var (
		currentTime = time.Now()
		err         = recover()
	)

	m.LastCPUDuration = currentTime.Sub(startAt)

	if err != nil {
		m.LastPanicTime = currentTime
		m.PanicNum++
	}

	context.Probe(context.Parent(), m)
	if err != nil {
		panic(err)
	}
}
