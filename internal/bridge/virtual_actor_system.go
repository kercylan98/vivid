package bridge

import (
	"time"

	"github.com/kercylan98/vivid"
)

// VirtualActorSystem 是虚拟 actor 系统的接口。
type VirtualActorSystem interface {
	vivid.ActorSystem
	TellWithSender

	// GetVirtualActorProvider 获取虚拟 Actor 的提供者。
	//
	// 参数：
	//   - kind: 虚拟 Actor 的种类。
	//
	// 返回：
	//   - 虚拟 Actor 的提供者，如果未找到则返回 nil。
	GetVirtualActorProvider(kind string) vivid.ActorProvider

	// AskWithSender 以特定发送者身份询问消息。
	AskWithSender(sender vivid.ActorRef, recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message]
}
