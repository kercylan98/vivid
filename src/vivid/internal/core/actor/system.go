package actor

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/wasteland/src/wasteland"
)

type System interface {
	LoggerProvider() log.Provider

	ResourceLocator() wasteland.ResourceLocator

	Run() error

	Stop() error

	Context() Context

	Find(target Ref) wasteland.ProcessHandler

	Register(ctx Context)

	Unregister(operator, target Ref)

	Registry() wasteland.ProcessRegistry
}
