package bridge

import (
	"time"

	"github.com/kercylan98/vivid"
)

// VirtualCoordinator 是虚拟 actor 协调器的接口。
type VirtualCoordinator interface {
	vivid.Actor
	TellVirtual(sender vivid.ActorRef, recipient vivid.ActorRef, message vivid.Message) error
	AskVirtual(sender vivid.ActorRef, recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message]
}
