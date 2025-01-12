package process

import "github.com/kercylan98/vivid/core"

type implOfProcess struct {
	id core.ID
}

func (p *implOfProcess) GetID() core.ID {
	return p.id
}

func (p *implOfProcess) Send(envelope core.Envelope) {

}

func (p *implOfProcess) Terminated() bool {
	return false
}

func (p *implOfProcess) OnTerminate(operator core.ID) {

}
