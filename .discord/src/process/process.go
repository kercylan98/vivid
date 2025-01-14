package process

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
)

type implOfProcess struct {
	id vivid2.ID
}

func (p *implOfProcess) GetID() vivid2.ID {
	return p.id
}

func (p *implOfProcess) Send(envelope vivid2.Envelope) {

}

func (p *implOfProcess) Terminated() bool {
	return false
}

func (p *implOfProcess) OnTerminate(operator vivid2.ID) {

}
