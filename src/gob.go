package vivid

import (
	"encoding/gob"
	"fmt"
)

// RegisterMessage 是 gob.Register 的快捷方式，用于注册消息类型
//   - 当需要跨网络传输且采用默认的 gob 编码器时，需要通过此方法注册你的消息类型
func RegisterMessage(message Message) {
	gob.Register(message)
}

// RegisterMessageName 是 gob.RegisterName 的快捷方式，用于注册消息类型
//   - 当需要跨网络传输且采用默认的 gob 编码器时，需要通过此方法注册你的消息类型
func RegisterMessageName(name string, message Message) {
	gob.RegisterName(name, message)
}

// registerInternalMessage 是 RegisterMessageName 的快捷方式，用于注册内部消息类型
func registerInternalMessage(message Message) {
	RegisterMessageName(fmt.Sprintf("vivid.%T", message), message)
}
