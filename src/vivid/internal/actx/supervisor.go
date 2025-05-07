package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"time"
)

func GetDefaultSupervisor(restartLimit int) actor.Supervisor {
	return &supervisor{restartLimit: restartLimit}
}

type supervisor struct {
	restartLimit int
}

func (d *supervisor) Decision(snapshot actor.AccidentSnapshot) {
	switch m := snapshot.GetMessage().(type) {
	case *actor.OnLaunch:
		if !m.Restarted() {
			// 如果 Actor 初始化失败，通常意味着它无法正常工作，因此直接停止。
			snapshot.Kill(snapshot.GetVictim(), "supervisor: OnLaunch failed")
		} else {
			// 否则尝试重启
			snapshot.ExponentialBackoffRestart(snapshot.GetVictim(), d.restartLimit, time.Millisecond*100, time.Second, 2.0, 0.5, "supervisor: try restart")
		}
	case *actor.OnKill:
		// TODO: 在该阶段发生失败时，Actor 的持久化可能由于网络等因素尚未完成。需要更合适的解决方案。
		if m.Poison {
			snapshot.PoisonKill(snapshot.GetVictim(), "supervisor: OnKill poison")
		} else {
			snapshot.Kill(snapshot.GetVictim(), "supervisor: OnKill")
		}
	default:
		// 其他情况下，尝试重启
		snapshot.ExponentialBackoffRestart(snapshot.GetVictim(), d.restartLimit, time.Millisecond*100, time.Second, 2.0, 0.5, "supervisor: try restart")
	}
}
