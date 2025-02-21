package vivid

import (
	"time"
)

// Supervisor 是一个监管者接口，用于对 Actor 异常情况进行监管策略的执行
type Supervisor interface {
	Decision(record AccidentRecord)
}

type SupervisorFn func(record AccidentRecord)

func (f SupervisorFn) Decision(record AccidentRecord) {
	f(record)
}

func getDefaultSupervisor(restartLimit int) Supervisor {
	return &defaultSupervisor{restartLimit: restartLimit}
}

type defaultSupervisor struct {
	restartLimit int
}

func (d *defaultSupervisor) Decision(record AccidentRecord) {
	switch m := record.GetMessage().(type) {
	case OnLaunch:
		if !m.Restarted() {
			// 如果 Actor 初始化失败，通常意味着它无法正常工作，因此直接停止。
			record.Kill(record.GetVictim(), "supervisor: OnLaunch failed")
		} else {
			// 否则尝试重启
			record.ExponentialBackoffRestart(record.GetVictim(), d.restartLimit, time.Millisecond*100, time.Second, 2.0, 0.5, "supervisor: try restart")
		}
	case OnKill:
		// TODO: 在该阶段发生失败时，Actor 的持久化可能由于网络等因素尚未完成。需要更合适的解决方案。
		if m.IsPoison() {
			record.PoisonKill(record.GetVictim(), "supervisor: OnKill poison")
		} else {
			record.Kill(record.GetVictim(), "supervisor: OnKill")
		}
	default:
		// 其他情况下，尝试重启
		record.ExponentialBackoffRestart(record.GetVictim(), d.restartLimit, time.Millisecond*100, time.Second, 2.0, 0.5, "supervisor: try restart")
	}
}
