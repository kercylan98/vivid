package actor

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
)

// Config 中描述了可被外部配置的 Actor 的配置项，它会在 Actor 创建时被传入，并在整个 Actor 的生命周期中被使用。
type Config struct {
	// Name 表示了一个 Actor 的名称，它将生成特定的资源标识符，同时也代表了一个 Actor 在其父级下的唯一性。
	//
	// 该名称在未设置时将会默认使用父级的 GUID 计数器生成一个唯一计数作为名称。
	Name string

	// LoggerProvider 表示了一个 Actor 的日志提供器，它将会被用于生成 Actor 的日志记录器。
	//
	// 可通过指定日志记录器并动态的调整返回的日志记录器的配置来实现对 Actor 日志的动态调整。
	//
	// 在未设置时将会默认使用父级的日志提供器作为 Actor 的日志提供器。
	LoggerProvider log.Provider

	// Mailbox 表示了一个 Actor 的邮箱，它将在 Actor 收到消息时用于存储消息。并在 Actor 空闲时从邮箱中取出消息进行处理。
	//
	// 在默认情况下，Actor 将会使用一个 FIFO 的 MPSC 邮箱。
	Mailbox mailbox.Mailbox

	// Dispatcher 表示了一个 Actor 的调度器，它将在 Actor 执行消息时用于完成对消息的调度。
	//
	// 在默认情况下，Actor 将会使用一个基于 Goroutine 实现的事件驱动消息调度器。
	Dispatcher mailbox.Dispatcher

	// Supervisor 表示了一个 Actor 的监管者，它将在 Actor 发生事故时用于处理事故。
	Supervisor Supervisor
}
