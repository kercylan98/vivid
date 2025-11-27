package vivid

import "net"

type (
	Message   = any
	ActorPath = string
	Behavior  = func(ctx ActorContext)
)

type (
	ActorRef interface {
		GetAddress() net.Addr
		GetPath() ActorPath
	}

	ActorContext interface {
		actorCore

		// System 用于获取当前 ActorContext 的 ActorSystem。
		System() ActorSystem
	}

	actorCore interface {
		// Parent 用于获取当前 ActorContext 的父 ActorRef，如果当前 ActorContext 是根 ActorContext，则返回 nil。
		Parent() ActorRef

		// Ref 用于获取当前 ActorContext 的 ActorRef。
		Ref() ActorRef

		// ActorOf 用于在当前 ActorContext 下创建子 Actor，并返回该子 Actor 的引用（ActorRef）。
		//
		// 参数:
		//   - actor   - 子 Actor 实例，需实现 Actor 接口；
		//   - options - 可选配置项（如 Actor 名称、邮箱等），通过可变参数传递；
		// 返回值:
		//   - ActorRef - 新建的子 Actor 引用；
		//
		// 注意：此方法非并发安全，不适用于多协程并发调用，通常情况下 Actor 的诞生是在其父的上下文中进行的，因此是天然线程安全的。
		ActorOf(actor Actor, options ...ActorOption) ActorRef
	}
)
