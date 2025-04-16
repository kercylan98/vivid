package core

import "github.com/kercylan98/wasteland/src/wasteland"

type (
	Address = wasteland.Address
	Path    = wasteland.Path
	Message = wasteland.Message
	URL     = string
)

const (
	UserMessage   = wasteland.MessagePriority(0)
	SystemMessage = wasteland.MessagePriority(1)
)
