package guard

import (
	"fmt"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

var (
	_ vivid.PrelaunchActor = &Actor{}
)

func NewActor(guardClosedSignal chan struct{}) *Actor {
	return &Actor{
		guardClosedSignal: guardClosedSignal,
	}
}

type Actor struct {
	guardClosedSignal chan struct{}
}

func (a *Actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	return nil
}

func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnKilled:
		a.onKilled(ctx, msg)
	case ves.DeathLetterEvent:
		a.onDeathLetter(ctx, msg)
	}
}

func (a *Actor) onDeathLetter(ctx vivid.ActorContext, msg ves.DeathLetterEvent) {
	ctx.EventStream().Publish(ctx, msg)

	ctx.Logger().Warn("death letter received",
		log.Time("time", msg.Time),
		log.Bool("system", msg.Envelope.System()),
		log.Any("sender", msg.Envelope.Sender()),
		log.Any("receiver", msg.Envelope.Receiver()),
		log.String("message_type", fmt.Sprintf("%T", msg.Envelope.Message())),
		log.Any("message", msg.Envelope.Message()),
	)
}

func (a *Actor) onKilled(ctx vivid.ActorContext, msg *vivid.OnKilled) {
	if msg.Ref.Equals(ctx.Ref()) {
		close(a.guardClosedSignal)
	}
}
