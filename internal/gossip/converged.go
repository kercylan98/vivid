package gossip

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
)

// maybeEmitConverged 当视图指纹连续两轮不变时向自身投递 Converged（仅投递一次，直到视图再次变化）。
func maybeEmitConverged(ctx vivid.ActorContext, a *Actor) {
	// ctx.Logger().Debug("try converged", log.String("status", a.info.Status.String()))
	fp := a.view.Fingerprint()
	if fp == a.lastViewFingerprint {
		a.stableRounds++
		// ctx.Logger().Debug("stable rounds", log.Int("stable_rounds", a.stableRounds))
		if a.stableRounds >= 3 && !a.convergedEmitted {
			// ctx.Logger().Debug("converged emitted")
			a.convergedEmitted = true
			ctx.Tell(ctx.Ref(), gossipmessages.NewConverged())
		}
		// else {
		// ctx.Logger().Debug("not converge", log.Int("stable_rounds", a.stableRounds), log.Bool("converged_emitted", a.convergedEmitted))
		// }
	} else {
		// ctx.Logger().Debug("view fingerprint changed")
		// ctx.Logger().Debug("before fingerprint", log.String("fingerprint", a.lastViewFingerprint))
		// ctx.Logger().Debug("latest fingerprint", log.String("fingerprint", fp))
		a.lastViewFingerprint = fp
		a.stableRounds = 0
		a.convergedEmitted = false
		a.convergenceStartedAt = time.Now() // 视图变化，重新计时收敛耗时
	}
}
