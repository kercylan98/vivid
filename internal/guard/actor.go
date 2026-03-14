package guard

import (
	"fmt"
	"sort"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

const (
	stopReason = "actor system stop"
)

var (
	_ vivid.PrelaunchActor = &Actor{}
)

// RegisterStopPriority 注册停止优先级
type RegisterStopPriority struct {
	ActorRef vivid.ActorRef
	Priority int // 优先级，值越大越先停止，但是始终在未注册的 Actor 之后停止
}

type Stop struct{}

func NewActor(guardClosedSignal chan struct{}) *Actor {
	return &Actor{
		guardClosedSignal: guardClosedSignal,
	}
}

type Actor struct {
	guardClosedSignal      chan struct{}
	registerStopPriorities []*RegisterStopPriority
	stopping               bool
}

func (a *Actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	return nil
}

func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *Stop:
		a.onStop(ctx)
	case *vivid.OnKilled:
		a.onKilled(ctx, msg)
	case *RegisterStopPriority:
		a.onRegisterStopPriority(ctx, msg)
	case ves.DeathLetterEvent:
		a.onDeathLetter(ctx, msg)
	}
}

func (a *Actor) onStop(ctx vivid.ActorContext) {
	if a.stopping {
		return
	}
	a.stopping = true

	// 先关闭未注册的 Actor
	// 已注册的在 [Actor.onKilled] 中关闭
	var onlyRegistered = true
	for _, child := range ctx.Children() {
		var registered = false
		for _, v := range a.registerStopPriorities {
			if registered = v.ActorRef.Equals(child); registered {
				break
			}
		}
		if registered {
			continue
		}
		onlyRegistered = false
		ctx.Kill(child, true, stopReason)
	}

	// 如果仅剩余已注册的 Actor 或没有子 Actor，则关闭自己
	if ctx.Children().Len() == 0 || onlyRegistered {
		ctx.Kill(ctx.Ref(), true, stopReason)
	}
}

func (a *Actor) onKilled(ctx vivid.ActorContext, msg *vivid.OnKilled) {
	if msg.Ref.Equals(ctx.Ref()) {
		close(a.guardClosedSignal)
		return
	}

	if a.stopping {
		// 如果剩余 Actor 仅包含已注册的，那么关闭已注册的
		for _, child := range ctx.Children() {
			var registered = false
			for _, v := range a.registerStopPriorities {
				if v.ActorRef.Equals(child) {
					registered = true
					break
				}
			}
			// 还存在未注册的 Actor，则不关闭
			if !registered {
				return
			}
		}

		// 关闭自己
		if len(a.registerStopPriorities) == 0 {
			ctx.Kill(ctx.Ref(), true, stopReason)
			return
		}

		next := a.registerStopPriorities[0]
		a.registerStopPriorities = a.registerStopPriorities[1:]
		ctx.Kill(next.ActorRef, true, stopReason)
	}
}

func (a *Actor) onRegisterStopPriority(_ vivid.ActorContext, msg *RegisterStopPriority) {
	a.registerStopPriorities = append(a.registerStopPriorities, msg)

	// 按照优先级降序，大的在前
	sort.Slice(a.registerStopPriorities, func(i, j int) bool {
		return a.registerStopPriorities[i].Priority > a.registerStopPriorities[j].Priority
	})
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
