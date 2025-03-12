package vivid

import "github.com/kercylan98/wasteland/src/wasteland"

type Message = wasteland.Message

type addressableMessage struct {
	Sender  ActorRef
	Message wasteland.Message
}

var (
	onLaunch = &OnLaunch{}
)

// LocalMessage is a mailboxMessageHandler that is sent to the local process.
type (
	OnLaunch struct{}
)

// RemoteMessage is a mailboxMessageHandler that is sent to the remote process.
type (
	_ = int
)
