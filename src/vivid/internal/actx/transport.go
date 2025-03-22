package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	futureImpl "github.com/kercylan98/vivid/src/vivid/internal/future"
	"github.com/kercylan98/wasteland/src/wasteland"
	"strconv"
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
	t.ctx.MetadataContext().System().Find(target).HandleMessage(t.ctx.MetadataContext().Ref(), priority, message)
}

func (t *Transport) Ask(target actor.Ref, priority wasteland.MessagePriority, message core.Message, timeout ...time.Duration) future.Future {
	d := time.Second
	if len(timeout) > 0 {
		d = timeout[0]
	}

	meta := t.ctx.MetadataContext()
	futureRef := meta.Ref().GenerateSub(strconv.FormatInt(t.ctx.RelationContext().NextGuid(), 10))
	f := futureImpl.New(meta.System().Registry(), futureRef, d)
	meta.System().Find(target).HandleMessage(f.GetID(), priority, message)
	return f
}

func (t *Transport) Reply(priority wasteland.MessagePriority, message core.Message) {
	t.ctx.TransportContext().Tell(t.ctx.MessageContext().Sender(), priority, message)
}
