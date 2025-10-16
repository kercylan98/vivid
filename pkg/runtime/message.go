package runtime

const (
	MessageTypeSystem MessageType = iota // 系统消息
	MessageTypeUser                      // 用户消息
)

type MessageType int8
type Message = any
