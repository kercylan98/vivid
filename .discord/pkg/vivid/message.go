package vivid

const (
	UserMessage MessageType = iota
	SystemMessage
)

type MessageType = uint8

type Message = any
