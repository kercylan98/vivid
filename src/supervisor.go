package vivid

// Supervisor 是一个监管者接口，用于对 Actor 异常情况进行监管策略的执行
type Supervisor interface {
	Decision(record AccidentRecord)
}

type SupervisorFn func(record AccidentRecord)

func (f SupervisorFn) Decision(record AccidentRecord) {
	f(record)
}
