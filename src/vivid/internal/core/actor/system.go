package actor

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/wasteland/src/wasteland"
)

type System interface {
	LoggerProvider() log.Provider

	Meta() wasteland.Meta

	Run() error

	Shutdown() error

	Context() Context

	Find(target Ref) wasteland.ProcessHandler

	Register(ctx Context)
}
