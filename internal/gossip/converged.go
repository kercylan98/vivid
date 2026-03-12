package gossip

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
)

// maybeEmitConverged 当视图指纹连续两轮不变时向自身投递 Converged（仅投递一次，直到视图再次变化）。
func maybeEmitConverged(ctx vivid.ActorContext, a *Actor) {
	fp := a.view.Fingerprint()
	if fp == a.lastViewFingerprint {
		a.stableRounds++
		if a.stableRounds >= 2 && !a.convergedEmitted {
			a.convergedEmitted = true
			ctx.Tell(ctx.Ref(), gossipmessages.NewConverged())
		}
	} else {
		a.lastViewFingerprint = fp
		a.stableRounds = 0
		a.convergedEmitted = false
	}
}
