package actor

import "github.com/kercylan98/wasteland/src/wasteland"

type ProcessContext interface {
	wasteland.ProcessLifecycle
	wasteland.Process
	wasteland.ProcessHandler
}
