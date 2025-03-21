package actor

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
)

type MessageContext interface {
	mailbox.Handler

	// Sender 获取消息发送者
	Sender() Ref

	// Message 获取消息
	Message() core.Message

	// OnReceiveImplant 注入消息处理器，以特定的消息触发 Actor.OnReceive 方法
	OnReceiveImplant(message core.Message)
}
