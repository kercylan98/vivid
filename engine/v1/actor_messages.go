package vivid

const (
    userMessage = iota
    systemMessage
)

type Message = any

var onLaunch = new(OnLaunch)
var onRestart = func() *OnLaunch {
    var m = new(OnLaunch)
    *m = 1
    return m
}()

type OnLaunch int32

func (m *OnLaunch) Restarted() bool {
    return *m == 1
}

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
