package actor

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	"github.com/kercylan98/wasteland/src/wasteland"
	"time"
)

type TransportContext interface {
	Tell(target Ref, priority wasteland.MessagePriority, message core.Message)

	Probe(target Ref, priority wasteland.MessagePriority, message core.Message)

	Ask(target Ref, priority wasteland.MessagePriority, message core.Message, timeout ...time.Duration) future.Future

	Reply(message core.Message)
}
