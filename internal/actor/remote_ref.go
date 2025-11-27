package actor

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/transparent"
)

var (
	_ vivid.ActorRef               = &remoteRef{}
	_ transparent.TransportContext = &remoteRef{}
)

func newRemoteRef(system *System, ref *Ref) *remoteRef {
	return &remoteRef{
		Ref:    ref,
		system: system,
	}
}

type remoteRef struct {
	*Ref
	system *System
}

func (r *remoteRef) HandleEnvelop(envelop vivid.Envelop) {
	// TODO: 发送消息到远程 Actor
	panic("not implemented")
}
