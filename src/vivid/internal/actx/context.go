package actx

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

var _ actor.Context = (*Context)(nil)

func New(system actor.System, config *actor.Config, ref actor.Ref, parent actor.Ref, provider actor.Provider) actor.Context {
	ctx := &Context{config: config}
	ctx.metadata = NewMetadata(system, ref, parent, config)
	ctx.relation = NewRelation()
	ctx.generate = NewGenerate(ctx, provider)
	ctx.process = NewProcess(ctx)
	ctx.message = NewMessage(ctx)
	return ctx
}

type Context struct {
	config   *actor.Config
	metadata actor.MetadataContext
	relation actor.RelationContext
	generate actor.GenerateContext
	process  actor.ProcessContext
	message  actor.MessageContext
}

func (c *Context) MessageContext() actor.MessageContext {
	return c.message
}

func (c *Context) LoggerProvider() log.Provider {
	return c.config.LoggerProvider
}

func (c *Context) MetadataContext() actor.MetadataContext {
	return c.metadata
}

func (c *Context) RelationContext() actor.RelationContext {
	return c.relation
}

func (c *Context) GenerateContext() actor.GenerateContext {
	return c.generate
}

func (c *Context) ProcessContext() actor.ProcessContext {
	return c.process
}
