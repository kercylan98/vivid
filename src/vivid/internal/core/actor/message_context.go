package actor

import "github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"

type MessageContext interface {
	mailbox.Handler
}
