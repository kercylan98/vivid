package main

var (
	_ Process        = (*implOfProcess)(nil)
	_ ProcessBuilder = (*implOfProcessBuilder)(nil)
)

var (
	processBuilder ProcessBuilder = &implOfProcessBuilder{}
)

func NewProcessBuilder() ProcessBuilder {
	return processBuilder
}

type ProcessBuilder interface {
	HostOf(host Host) Process
}

type implOfProcessBuilder struct{}

func (i *implOfProcessBuilder) HostOf(host Host) Process {
	return &implOfProcess{
		id: NewIDBuilder().Build(host, "/"),
	}
}

// Process 是一个抽象概念，它代表了一个进程
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

type implOfProcess struct {
	id ID
}

func (p *implOfProcess) GetID() ID {
	return p.id
}

func (p *implOfProcess) Send(envelope Envelope) {

}

func (p *implOfProcess) Terminated() bool {
	return false
}

func (p *implOfProcess) OnTerminate(operator ID) {

}
