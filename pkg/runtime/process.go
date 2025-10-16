package runtime

type Process interface {
	OnMessage(sender *ProcessID, messageType MessageType, message Message) error
}
