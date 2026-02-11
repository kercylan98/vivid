package cluster

import "github.com/kercylan98/vivid"

type singletonForwardedMessage struct {
	sender      vivid.ActorRef
	message     vivid.Message
	senderAddr  string // 反序列化后填充，用于延迟解析 sender
	senderPath  string
}
