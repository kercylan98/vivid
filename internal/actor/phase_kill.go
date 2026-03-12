package actor

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

func newPhaseKill(c <-chan struct{}, timeout time.Duration, behavior vivid.Behavior) *phaseKill {
	return &phaseKill{
		c:        c,
		timeout:  timeout,
		behavior: behavior,
	}
}

type phaseKill struct {
	c        <-chan struct{}
	timeout  time.Duration
	behavior vivid.Behavior
	envelope vivid.Envelop
}

func (p *phaseKill) apply(ctx *Context, envelope vivid.Envelop) {
	p.envelope = envelope
	go func() {
		timer := time.NewTimer(p.timeout)
		defer timer.Stop()
		select {
		case <-p.c:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			ctx.Logger().Warn("phase kill timeout", log.String("path", ctx.ref.GetPath()), log.Duration("timeout", p.timeout))
		}
		ctx.TellSelf(p)
	}()
}
