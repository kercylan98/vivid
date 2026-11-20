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

func NewActor() *Actor {
	return &Actor{}
}

type Actor struct {
	registerStopPriorities map[vivid.ActorPath]RegisterStopPriority
	pendingStops           map[vivid.ActorPath]pendingStop
	stopPriorities         []RegisterStopPriority
	remainingUnregistered  int
	stoppingRegistered     vivid.ActorRef
	stopping               bool
}

type pendingStop struct {
	ref        vivid.ActorRef
	registered bool
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

	// 停止前已开始的根级创建已完成，只保存一次名单，避免逐个终止时反复复制子节点。
	children := ctx.Children()
	if len(children) > 0 {
		a.pendingStops = make(map[vivid.ActorPath]pendingStop, len(children))
	}
	for _, child := range children {
		priority, registered := a.registerStopPriorities[child.GetPath()]
		registered = registered && priority.ActorRef.Equals(child)
		a.pendingStops[child.GetPath()] = pendingStop{ref: child, registered: registered}
		if registered {
			a.stopPriorities = append(a.stopPriorities, priority)
		} else {
			a.remainingUnregistered++
		}
	}
	a.registerStopPriorities = nil
	sort.Slice(a.stopPriorities, func(i, j int) bool {
		return a.stopPriorities[i].Priority > a.stopPriorities[j].Priority
	})

	// 使用系统终止消息，普通邮箱暂停不应阻塞生命周期收尾。
	for _, child := range children {
		if !a.pendingStops[child.GetPath()].registered {
			ctx.Kill(child, false, stopReason)
		}
	}
	a.killNextRegistered(ctx)
}

func (a *Actor) onKilled(ctx vivid.ActorContext, msg *vivid.OnKilled) {
	if msg.Ref.Equals(ctx.Ref()) {
		return
	}
	if !a.stopping {
		if priority, ok := a.registerStopPriorities[msg.Ref.GetPath()]; ok && priority.ActorRef.Equals(msg.Ref) {
			delete(a.registerStopPriorities, msg.Ref.GetPath())
		}
		return
	}

	// 仅接纳名单内且身份匹配的通知，重复或未知通知不能提前推进关闭。
	pending, ok := a.pendingStops[msg.Ref.GetPath()]
	if !ok || !pending.ref.Equals(msg.Ref) {
		return
	}
	delete(a.pendingStops, msg.Ref.GetPath())
	if !pending.registered {
		a.remainingUnregistered--
	} else if a.stoppingRegistered != nil && a.stoppingRegistered.Equals(msg.Ref) {
		a.stoppingRegistered = nil
	}
	a.killNextRegistered(ctx)
}

func (a *Actor) killNextRegistered(ctx vivid.ActorContext) {
	if a.remainingUnregistered > 0 || a.stoppingRegistered != nil {
		return
	}
	for len(a.stopPriorities) > 0 {
		next := a.stopPriorities[0]
		a.stopPriorities[0] = RegisterStopPriority{}
		a.stopPriorities = a.stopPriorities[1:]
		if pending, ok := a.pendingStops[next.ActorRef.GetPath()]; ok && pending.ref.Equals(next.ActorRef) {
			a.stoppingRegistered = pending.ref
			ctx.Kill(pending.ref, false, stopReason)
			return
		}
	}
	a.pendingStops = nil
	a.stopPriorities = nil
	ctx.Kill(ctx.Ref(), false, stopReason)
}

func (a *Actor) onRegisterStopPriority(_ vivid.ActorContext, msg *RegisterStopPriority) {
	if a.stopping || msg.ActorRef == nil {
		return
	}
	if a.registerStopPriorities == nil {
		a.registerStopPriorities = make(map[vivid.ActorPath]RegisterStopPriority)
	}
	// 复制登记值，同一路径重复登记更新优先级；停止开始后不再修改关闭顺序。
	a.registerStopPriorities[msg.ActorRef.GetPath()] = *msg
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
