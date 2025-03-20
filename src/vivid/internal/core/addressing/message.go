package addressing

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

func init() {
	gob.RegisterName("*addressing.Message", &Message{})
}

func NewMessage(sender actor.Ref, message core.Message) *Message {
	return &Message{
		Sender:  sender,
		Message: message,
	}
}

// Message 是可被寻址的消息包装
type Message struct {
	Sender  actor.Ref
	Message core.Message
}
