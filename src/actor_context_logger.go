package vivid

import "github.com/kercylan98/go-log/log"

var (
	_ actorContextLoggerInternal = (*actorContextLoggerImpl)(nil)
)

func newActorContextLoggerImpl(ctx ActorContext) *actorContextLoggerImpl {
	return &actorContextLoggerImpl{
		ActorContext: ctx,
	}
}

type actorContextLoggerImpl struct {
	ActorContext
}

func (ctx *actorContextLoggerImpl) GetLoggerProvider() log.Provider {
	return ctx.getConfig().FetchLoggerProvider()
}

func (ctx *actorContextLoggerImpl) Logger() log.Logger {
	return ctx.getConfig().FetchLogger()
}
