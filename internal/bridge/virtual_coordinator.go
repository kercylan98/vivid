package bridge

import "github.com/kercylan98/vivid"

// VirtualCoordinator 是虚拟 actor 协调器的接口。
type VirtualCoordinator interface {
	vivid.Actor
	TellVirtual(sender vivid.ActorRef, recipient vivid.ActorRef, message vivid.Message) error

	// ResolveVirtual 解析并按需激活目标；请求登记与投递由调用方负责。
	ResolveVirtual(recipient vivid.ActorRef) (vivid.ActorRef, error)
}
