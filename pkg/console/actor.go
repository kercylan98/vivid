package console

import (
	"context"
	"reflect"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/console/internal/server"
	"github.com/kercylan98/vivid/pkg/console/internal/store"
	"github.com/kercylan98/vivid/pkg/ves"
)

var (
	_ vivid.Actor          = (*actor)(nil)
	_ vivid.PrelaunchActor = (*actor)(nil)
)

func Serve(system vivid.ActorSystem, addr string, options ...Option) error {
	opts := NewOptions(options...)
	recentStore := store.NewRecentStore()
	srv := server.NewServer(addr, opts.ReadTimeout, opts.WriteTimeout, system, recentStore)
	actor := &actor{
		server: srv,
		store:  recentStore,
	}
	_, err := system.ActorOf(actor, vivid.WithActorName("@console"))
	return err
}

type actor struct {
	server *server.Server
	store  *store.RecentStore
}

func (a *actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	return nil
}

func (a *actor) OnReceive(ctx vivid.ActorContext) {
	switch m := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case error:
		ctx.Kill(ctx.Ref(), true, m.Error())
	case *vivid.OnKill:
		a.server.Shutdown(context.Background())
	default:
		if streamEvent, ok := m.(vivid.StreamEvent); ok {
			a.onStreamEvent(ctx, streamEvent)
		}
	}
}

func refPath(ref vivid.ActorRef) string {
	if ref == nil {
		return ""
	}
	return ref.GetPath()
}

func (a *actor) onStreamEvent(ctx vivid.ActorContext, event vivid.StreamEvent) {
	now := time.Now()
	switch e := event.(type) {
	case ves.DeathLetterEvent:
		sender := ""
		receiver := ""
		msgType := ""
		if e.Envelope != nil {
			if s := e.Envelope.Sender(); s != nil {
				sender = s.String()
			}
			if r := e.Envelope.Receiver(); r != nil {
				receiver = r.String()
			}
			if e.Envelope.Message() != nil {
				msgType = reflect.TypeOf(e.Envelope.Message()).String()
			}
		}
		t := e.Time
		if t.IsZero() {
			t = now
		}
		a.store.AddDeathLetter(sender, receiver, msgType, t)
	case ves.ActorSpawnedEvent:
		a.store.AddEvent("ActorSpawned", refPath(e.ActorRef), now)
	case ves.ActorLaunchedEvent:
		a.store.AddEvent("ActorLaunched", refPath(e.ActorRef), now)
	case ves.ActorKilledEvent:
		a.store.AddEvent("ActorKilled", refPath(e.ActorRef), now)
	case ves.ActorFailedEvent:
		a.store.AddEvent("ActorFailed", refPath(e.ActorRef), now)
	case ves.ActorRestartingEvent:
		a.store.AddEvent("ActorRestarting", refPath(e.ActorRef)+": "+e.Reason, now)
	case ves.ActorRestartedEvent:
		a.store.AddEvent("ActorRestarted", refPath(e.ActorRef), now)
	}
}

func (a *actor) onLaunch(ctx vivid.ActorContext) {
	es := ctx.EventStream()
	es.Subscribe(ctx, ves.DeathLetterEvent{})
	es.Subscribe(ctx, ves.ActorSpawnedEvent{})
	es.Subscribe(ctx, ves.ActorLaunchedEvent{})
	es.Subscribe(ctx, ves.ActorKilledEvent{})
	es.Subscribe(ctx, ves.ActorFailedEvent{})
	es.Subscribe(ctx, ves.ActorRestartingEvent{})
	es.Subscribe(ctx, ves.ActorRestartedEvent{})
	ctx.Entrust(vivid.ForeverEntrust, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		return nil, a.server.Start()
	})).PipeTo(ctx.Ref().ToActorRefs())
}
