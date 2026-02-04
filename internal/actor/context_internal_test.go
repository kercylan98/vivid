package actor

import "github.com/kercylan98/vivid"

// AdvancedTell 高级发送消息，支持系统消息和普通消息，并且支持设置发送者。
// 该接口用于单元测试对外暴露
func (c *Context) AdvancedTell(system bool, recipient vivid.ActorRef, message vivid.Message) {
	c.tell(system, recipient, message)
}
