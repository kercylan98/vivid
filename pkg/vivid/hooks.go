package vivid

import (
	"reflect"
	"time"
)

var (
	// actorLaunchHookType ActorLaunchHook.OnActorLaunch(ctx ActorContext)
	actorLaunchHookType = reflect.TypeOf((*ActorLaunchHook)(nil)).Elem()
	// actorKillHookType ActorKillHook.OnActorKill(ctx ActorContext, message *OnKill)
	actorKillHookType = reflect.TypeOf((*ActorKillHook)(nil)).Elem()
	// actorKilledHookType ActorKilledHook.OnActorKilled(message *OnKilled)
	actorKilledHookType = reflect.TypeOf((*ActorKilledHook)(nil)).Elem()
	// actorMailboxPushSystemMessageBeforeHookType ActorMailboxPushSystemMessageBeforeHook.OnActorMailboxPushSystemMessageBefore(ref ActorRef, message Message)
	actorMailboxPushSystemMessageBeforeHookType = reflect.TypeOf((*ActorMailboxPushSystemMessageBeforeHook)(nil)).Elem()
	// actorMailboxPushUserMessageBeforeHookType ActorMailboxPushUserMessageBeforeHook.OnActorMailboxPushUserMessageBefore(ref ActorRef, message Message)
	actorMailboxPushUserMessageBeforeHookType = reflect.TypeOf((*ActorMailboxPushUserMessageBeforeHook)(nil)).Elem()
	// actorHandleSystemMessageBeforeHookType ActorHandleSystemMessageBeforeHook.OnActorHandleSystemMessageBefore(sender, receiver ActorRef, message Message)
	actorHandleSystemMessageBeforeHookType = reflect.TypeOf((*ActorHandleSystemMessageBeforeHook)(nil)).Elem()
	// actorHandleUserMessageBeforeHookType ActorHandleUserMessageBeforeHook.OnActorHandleUserMessageBefore(sender, receiver ActorRef, message Message)
	actorHandleUserMessageBeforeHookType = reflect.TypeOf((*ActorHandleUserMessageBeforeHook)(nil)).Elem()
	// actorHandleSystemMessageAfterHookType ActorHandleSystemMessageAfterHook.OnActorHandleSystemMessageAfter(sender, receiver ActorRef, message Message, duration time.Duration)
	actorHandleSystemMessageAfterHookType = reflect.TypeOf((*ActorHandleSystemMessageAfterHook)(nil)).Elem()
	// actorHandleUserMessageAfterHookType ActorHandleUserMessageAfterHook.OnActorHandleUserMessageAfter(sender, receiver ActorRef, message Message, duration time.Duration)
	actorHandleUserMessageAfterHookType = reflect.TypeOf((*ActorHandleUserMessageAfterHook)(nil)).Elem()

	hookTypes = map[hookType]string{
		actorLaunchHookType:                         "OnActorLaunch",
		actorKillHookType:                           "OnActorKill",
		actorKilledHookType:                         "OnActorKilled",
		actorMailboxPushSystemMessageBeforeHookType: "OnActorMailboxPushSystemMessageBefore",
		actorMailboxPushUserMessageBeforeHookType:   "OnActorMailboxPushUserMessageBefore",
		actorHandleSystemMessageBeforeHookType:      "OnActorHandleSystemMessageBefore",
		actorHandleUserMessageBeforeHookType:        "OnActorHandleUserMessageBefore",
		actorHandleSystemMessageAfterHookType:       "OnActorHandleSystemMessageAfter",
		actorHandleUserMessageAfterHookType:         "OnActorHandleUserMessageAfter",
	}
)

type HookProvider interface {
	hooks() []Hook
}

type HookProviderFN func() []Hook

func (fn HookProviderFN) hooks() []Hook {
	return fn()
}

func isRegisteredHookType(t hookType) bool {
	_, ok := hookTypes[t]
	return ok
}

func getHookTypeMethodName(t hookType) string {
	return hookTypes[t]
}

type hookType = reflect.Type

type Hook interface {
	hook()
}

type HookCore struct{}

func (HookCore) hook() {}

type (
	// ActorLaunchHookFN 是 ActorLaunchHook 的函数类型。
	ActorLaunchHookFN func(ctx ActorContext)
	// ActorLaunchHook 是一个 Actor 启动钩子接口，它允许在 Actor 启动时执行一些操作。
	ActorLaunchHook interface {
		OnActorLaunch(ctx ActorContext)
	}
)

func (fn ActorLaunchHookFN) hook() {}

func (fn ActorLaunchHookFN) OnActorLaunch(ctx ActorContext) {
	fn(ctx)
}

type (
	// ActorKillHookFN 是 ActorKillHook 的函数类型。
	ActorKillHookFN func(ctx ActorContext, message *OnKill)
	// ActorKillHook 是一个 Actor 结束钩子接口，它允许在 Actor 结束时执行一些操作。
	ActorKillHook interface {
		OnActorKill(ctx ActorContext, message *OnKill)
	}
)

func (fn ActorKillHookFN) hook() {}

func (fn ActorKillHookFN) OnActorKill(ctx ActorContext, message *OnKill) {
	fn(ctx, message)
}

type (
	// ActorKilledHookFN 是 ActorKilledHook 的函数类型。
	ActorKilledHookFN func(message *OnKilled)
	// ActorKilledHook 是一个 Actor 结束钩子接口，它允许在 Actor 结束时执行一些操作。
	ActorKilledHook interface {
		OnActorKilled(message *OnKilled)
	}
)

func (fn ActorKilledHookFN) hook() {}

func (fn ActorKilledHookFN) OnActorKilled(message *OnKilled) {
	fn(message)
}

type (
	// ActorMailboxPushSystemMessageBeforeHookFN 是 ActorMailboxPushSystemMessageBeforeHook 的函数类型。
	ActorMailboxPushSystemMessageBeforeHookFN func(ref ActorRef, message Message)
	// ActorMailboxPushSystemMessageBeforeHook 是一个 Actor 邮箱钩子接口，它允许在 Actor 邮箱中推送系统消息之前执行一些操作。
	ActorMailboxPushSystemMessageBeforeHook interface {
		OnActorMailboxPushSystemMessageBefore(ref ActorRef, message Message)
	}
)

func (fn ActorMailboxPushSystemMessageBeforeHookFN) hook() {}

func (fn ActorMailboxPushSystemMessageBeforeHookFN) OnActorMailboxPushSystemMessageBefore(ref ActorRef, message Message) {
	fn(ref, message)
}

type (
	// ActorMailboxPushUserMessageBeforeHookFN 是 ActorMailboxPushUserMessageBeforeHook 的函数类型。
	ActorMailboxPushUserMessageBeforeHookFN func(ref ActorRef, message Message)
	// ActorMailboxPushUserMessageBeforeHook 是一个 Actor 邮箱钩子接口，它允许在 Actor 邮箱中推送用户消息之前执行一些操作。
	ActorMailboxPushUserMessageBeforeHook interface {
		OnActorMailboxPushUserMessageBefore(ref ActorRef, message Message)
	}
)

func (fn ActorMailboxPushUserMessageBeforeHookFN) hook() {}

func (fn ActorMailboxPushUserMessageBeforeHookFN) OnActorMailboxPushUserMessageBefore(ref ActorRef, message Message) {
	fn(ref, message)
}

type (
	// ActorHandleSystemMessageBeforeHookFN 是 ActorHandleSystemMessageBeforeHook 的函数类型。
	ActorHandleSystemMessageBeforeHookFN func(sender ActorRef, receiver ActorRef, message Message)
	// ActorHandleSystemMessageBeforeHook 是一个 Actor 邮箱钩子接口，它允许在 Actor 处理系统消息之前执行一些操作。
	ActorHandleSystemMessageBeforeHook interface {
		OnActorHandleSystemMessageBefore(sender ActorRef, receiver ActorRef, message Message)
	}
)

func (fn ActorHandleSystemMessageBeforeHookFN) hook() {}

func (fn ActorHandleSystemMessageBeforeHookFN) OnActorHandleSystemMessageBefore(sender ActorRef, receiver ActorRef, message Message) {
	fn(sender, receiver, message)
}

type (
	ActorHandleUserMessageBeforeHookFN func(sender ActorRef, receiver ActorRef, message Message)
	// ActorHandleUserMessageBeforeHook 是一个 Actor 邮箱钩子接口，它允许在 Actor 处理用户消息之前执行一些操作。
	ActorHandleUserMessageBeforeHook interface {
		OnActorHandleUserMessageBefore(sender ActorRef, receiver ActorRef, message Message)
	}
)

func (fn ActorHandleUserMessageBeforeHookFN) hook() {}

func (fn ActorHandleUserMessageBeforeHookFN) OnActorHandleUserMessageBefore(sender ActorRef, receiver ActorRef, message Message) {
	fn(sender, receiver, message)
}

type (
	// ActorHandleSystemMessageAfterHookFN 是 ActorHandleSystemMessageAfterHook 的函数类型。
	ActorHandleSystemMessageAfterHookFN func(sender ActorRef, receiver ActorRef, message Message, duration time.Duration)
	// ActorHandleSystemMessageAfterHook 是一个 Actor 消息处理钩子接口，允许在 Actor 处理系统消息之后执行操作。
	ActorHandleSystemMessageAfterHook interface {
		OnActorHandleSystemMessageAfter(sender ActorRef, receiver ActorRef, message Message, duration time.Duration)
	}
)

func (fn ActorHandleSystemMessageAfterHookFN) hook() {}

func (fn ActorHandleSystemMessageAfterHookFN) OnActorHandleSystemMessageAfter(sender ActorRef, receiver ActorRef, message Message, duration time.Duration) {
	fn(sender, receiver, message, duration)
}

type (
	// ActorHandleUserMessageAfterHookFN 是 ActorHandleUserMessageAfterHook 的函数类型。
	ActorHandleUserMessageAfterHookFN func(sender ActorRef, receiver ActorRef, message Message, duration time.Duration)
	// ActorHandleUserMessageAfterHook 是一个 Actor 消息处理钩子接口，允许在 Actor 处理用户消息之后执行操作。
	ActorHandleUserMessageAfterHook interface {
		OnActorHandleUserMessageAfter(sender ActorRef, receiver ActorRef, message Message, duration time.Duration)
	}
)

func (fn ActorHandleUserMessageAfterHookFN) hook() {}

func (fn ActorHandleUserMessageAfterHookFN) OnActorHandleUserMessageAfter(sender ActorRef, receiver ActorRef, message Message, duration time.Duration) {
	fn(sender, receiver, message, duration)
}
