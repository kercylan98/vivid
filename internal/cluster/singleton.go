package cluster

import (
	"fmt"
	"github.com/kercylan98/vivid"
)

// singletonForwardedMessage 由单例代理发往单例的转发消息，携带原始 sender 与 message。
// 跨节点序列化时 sender 不可用，由 senderAddr+senderPath 在收端延迟解析。
type singletonForwardedMessage struct {
	sender     vivid.ActorRef
	message    vivid.Message
	senderAddr string // 序列化时写入，反序列化后用于解析 sender
	senderPath string
}

func (m *singletonForwardedMessage) String() string {
	if m.sender != nil {
		return fmt.Sprintf("forwarded(sender=%s, message=%T)", m.sender, m.message)
	}
	return fmt.Sprintf("forwarded(addr=%s path=%s, message=%T)", m.senderAddr, m.senderPath, m.message)
}
