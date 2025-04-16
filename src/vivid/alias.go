package vivid

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	"github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
)

type (
	Address    = core.Address
	Path       = core.Path
	Message    = core.Message
	Future     = future.Future
	Mailbox    = mailbox.Mailbox
	Dispatcher = mailbox.Dispatcher
)
