package vivid

import "sync"

var messageWrapperPool = sync.Pool{New: func() any { return &messageWrapper{} }}

type messageWrapper struct {
    sender  ActorRef
    message Message
}

func wrapMessage(sender ActorRef, message Message) *messageWrapper {
    wrapper := messageWrapperPool.Get().(*messageWrapper)
    wrapper.sender = sender
    wrapper.message = message
    return wrapper
}

func unwrapMessage(m any) (sender ActorRef, message Message) {
    switch v := m.(type) {
    case *messageWrapper:
        sender, message = v.sender, v.message
        messageWrapperPool.Put(v)
        return
    default:
        return nil, m
    }
}
