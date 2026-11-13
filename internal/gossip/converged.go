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

// resetViewStability 成员更新或阶段变化后立即作废原有观察，不等待下一个周期。
func (a *Actor) resetViewStability() {
	a.lastViewFingerprint, a.stableRounds = "", 0
	a.viewStableEmitted, a.pendingViewStable = false, nil
	a.stabilityStartedAt = time.Time{}
}

// sampleViewStability 建立基线后，连续三个采样间隔不变时通知自身，同一视图只投递一次。
func (a *Actor) sampleViewStability(ctx vivid.ActorContext, now time.Time) {
	if !a.lastStableSample.IsZero() && now.Sub(a.lastStableSample) < a.opts.GossipInterval {
		return
	}
	// 调度积压时只记录本次观察，不补算错过的周期。
	a.lastStableSample = now
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
		a.stabilityStartedAt = now
	}
}
