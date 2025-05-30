package actor

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/wasteland/src/wasteland"
)

type System interface {
	LoggerProvider() log.Provider

	ResourceLocator() wasteland.ResourceLocator

	Run() error

	Stop() error

	PoisonStop() error

	Context() Context

	Find(target Ref) wasteland.ProcessHandler

	Register(ctx Context)

	Unregister(operator, target Ref)

	Registry() wasteland.ProcessRegistry

	GetTimingWheel() timing.Wheel

	// SetGlobalMonitoring 设置全局监控实例
	SetGlobalMonitoring(monitoring interface{})

	// GetGlobalMonitoring 获取全局监控实例
	GetGlobalMonitoring() interface{}
}
