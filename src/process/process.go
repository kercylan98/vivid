package process

import (
	"github.com/kercylan98/vivid/pkg/vivid"
)

type implOfProcess struct {
	id vivid.ID
}

func (p *implOfProcess) GetID() vivid.ID {
	return p.id
}

func (p *implOfProcess) Send(envelope vivid.Envelope) {

}

func (p *implOfProcess) Terminated() bool {
	return false
}

func (p *implOfProcess) OnTerminate(operator vivid.ID) {

}
