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

	// HandleWith 注入消息并以特定的消息触发 Actor.OnReceive 方法，在处理完成后恢复当前的消息
	HandleWith(message core.Message)
}
