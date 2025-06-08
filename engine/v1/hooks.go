package vivid

import "reflect"

var (
	// actorLaunchHookType ActorLaunchHook.OnActorLaunch(ctx ActorContext)
	actorLaunchHookType = reflect.TypeOf((*ActorLaunchHook)(nil)).Elem()
	// actorKillHookType ActorKillHook.OnActorKill(ctx ActorContext, message *OnKill)
	actorKillHookType = reflect.TypeOf((*ActorKillHook)(nil)).Elem()
	// actorKilledHookType ActorKilledHook.OnActorKilled(message *OnKilled)
	actorKilledHookType = reflect.TypeOf((*ActorKilledHook)(nil)).Elem()

	hookTypes = map[hookType]string{
		actorLaunchHookType: "OnActorLaunch",
		actorKillHookType:   "OnActorKill",
		actorKilledHookType: "OnActorKilled",
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
