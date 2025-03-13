package mailbox

import "github.com/kercylan98/vivid/src/vivid/internal/core"

type Mailbox interface {
	Handler
	Initialize(dispatcher Dispatcher, handler Handler)
	Suspend()
	Resume()
}

type Handler interface {
	HandleSystemMessage(message core.Message)
	HandleUserMessage(message core.Message)
}
