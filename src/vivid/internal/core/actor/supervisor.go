package actor

// Supervisor 是一个监管者接口，用于对 Actor 异常情况进行监管策略的执行
type Supervisor interface {
    Decision(snapshot AccidentSnapshot)
}

type SupervisorFN func(snapshot AccidentSnapshot)

func (f SupervisorFN) Decision(snapshot AccidentSnapshot) {
    f(snapshot)
}
