package actor

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	"github.com/kercylan98/wasteland/src/wasteland"
	"time"
)

type TransportContext interface {
	Tell(target Ref, priority wasteland.MessagePriority, message core.Message)

	Probe(target Ref, priority wasteland.MessagePriority, message core.Message)

	Ask(target Ref, priority wasteland.MessagePriority, message core.Message, timeout ...time.Duration) future.Future

	Reply(priority wasteland.MessagePriority, message core.Message)

	// Ping 向目标 Actor 发送 ping 消息并等待 pong 响应。
	// 它直接返回 Pong 结构体和可能的错误。
	// 如果目标 Actor 不可达或者超时，将返回错误。
	Ping(target Ref, timeout ...time.Duration) (*OnPong, error)
	HandlePing(msg *OnPing)
	HandlePong(msg *OnPong, sender Ref)
}
