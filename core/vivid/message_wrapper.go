package vivid

type messageWrapper struct {
	sender  ActorRef
	message Message
}

func wrapMessage(sender ActorRef, message Message) *messageWrapper {
	return &messageWrapper{sender, message}
}

func unwrapMessage(m any) (sender ActorRef, message Message) {
	switch v := m.(type) {
	case *messageWrapper:
		return v.sender, v.message
	default:
		return nil, m
	}
}
