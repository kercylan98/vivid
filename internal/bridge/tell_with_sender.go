package bridge

import "github.com/kercylan98/vivid"

type TellWithSender interface {
	// TellWithSender 以特定发送者身份发送消息。
	TellWithSender(sender vivid.ActorRef, recipient vivid.ActorRef, message vivid.Message)
}
