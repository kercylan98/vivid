package vivid

func newRootProcess(host Host) *rootProcess {
	return &rootProcess{
		id: GetIDBuilder().RootOf(host),
	}
}

type Process interface {
	// GetID 返回这个进程的唯一标识
	GetID() ID

	// Send 将包装后的消息交由进程处理
	Send(envelope Envelope)

	// Terminated 检查进程是否已经终止
	Terminated() bool

	// OnTerminate 当进程被终止时调用，参数是发起终止的进程 ID
	OnTerminate(operator ID)
}

type rootProcess struct {
	id ID
}

func (r *rootProcess) GetID() ID {
	return r.id
}

func (r *rootProcess) Send(envelope Envelope) {

}

func (r *rootProcess) Terminated() bool {
	return false
}

func (r *rootProcess) OnTerminate(operator ID) {

}
