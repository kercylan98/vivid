package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	"github.com/kercylan98/wasteland/src/wasteland"
	"time"
)

var _ actor.TransportContext = (*Transport)(nil)

func NewTransport(ctx actor.Context) *Transport {
	return &Transport{
		ctx: ctx,
	}
}

type Transport struct {
	ctx actor.Context
}

func (t *Transport) Tell(target actor.Ref, priority wasteland.MessagePriority, message core.Message) {
	t.ctx.MetadataContext().System().Find(target).HandleMessage(nil, priority, message)
}

func (t *Transport) Probe(target actor.Ref, priority wasteland.MessagePriority, message core.Message) {

}

func (t *Transport) Ask(target actor.Ref, priority wasteland.MessagePriority, message core.Message, timeout ...time.Duration) future.Future {
	return nil
}

func (t *Transport) Reply(message core.Message) {

}
