package vividtemp

import "github.com/kercylan98/go-log/log"

func NewActorSystemConfig() ActorSystemConfiguration {
	return ActorSystemConfiguration{
		ActorSystemRuntimeConfiguration: ActorSystemRuntimeConfiguration{
			LoggerProvider: log.ProviderFn(log.GetDefault),
		},
	}
}

type ActorSystemConfiguration struct {
	ActorSystemRuntimeConfiguration
	Address string // Actor 系统的地址
}

// ActorSystemRuntimeConfiguration 是 Actor 系统的运行时配置，它们使用的内容可能随时变化
type ActorSystemRuntimeConfiguration struct {
	LoggerProvider log.Provider // 日志记录器提供器
}
