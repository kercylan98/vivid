package vivid

import "github.com/kercylan98/go-log/log"

func NewActorConfig(ctx ActorContext) ActorConfiguration {
	return ActorConfiguration{
		Mailbox:                   newMailbox(),
		Dispatcher:                newDispatcher(),
		ActorRuntimeConfiguration: ActorRuntimeConfiguration{},
	}
}

type ActorConfiguration struct {
	ActorRuntimeConfiguration
	Name       string     // Actor 的名称
	Mailbox    Mailbox    // Actor 使用的邮箱
	Dispatcher Dispatcher // Actor 使用的调度器
}

type ActorRuntimeConfiguration struct {
	LoggerProvider log.Provider
}
