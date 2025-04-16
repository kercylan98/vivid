package actor

// Supervisor 是一个监管者接口，用于对 Actor 异常情况进行监管策略的执行
type Supervisor interface {
	Decision(snapshot Snapshot)
}

type SupervisorFN func(snapshot Snapshot)

func (f SupervisorFN) Decision(snapshot Snapshot) {
	f(snapshot)
}
