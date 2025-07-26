package processor

type MessageWrapper struct {
	sender  UnitIdentifier
	message any
}

func WrapMessage(sender UnitIdentifier, message any) *MessageWrapper {
	wrapper := &MessageWrapper{}
	wrapper.sender = sender
	wrapper.message = message
	return wrapper
}

func UnwrapMessage(m any) (sender UnitIdentifier, message any) {
	switch v := m.(type) {
	case *MessageWrapper:
		sender, message = v.sender, v.message
		return
	default:
		return nil, m
	}
}
