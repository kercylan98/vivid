package gossip

import (
	"time"

	"github.com/kercylan98/vivid"
)

// viewStable 仅用于本地邮箱，绑定产生通知时的实例与成员视图，不表示远端确认。
type viewStable struct {
	instanceID  string
	fingerprint string
}

// maybeEmitViewStable 连续三次观察未变化时通知自身，同一视图只投递一次。
func maybeEmitViewStable(ctx vivid.ActorContext, a *Actor) {
	fp := a.view.Fingerprint()
	if fp == a.lastViewFingerprint {
		a.stableRounds++
		if a.stableRounds >= 3 && !a.viewStableEmitted {
			a.viewStableEmitted = true
			a.pendingViewStable = &viewStable{instanceID: a.info.ID(), fingerprint: fp}
			ctx.Tell(ctx.Ref(), a.pendingViewStable)
		}
	} else {
		a.lastViewFingerprint = fp
		a.stableRounds = 0
		a.viewStableEmitted = false
		a.pendingViewStable = nil
		a.stabilityStartedAt = time.Now()
	}
}
