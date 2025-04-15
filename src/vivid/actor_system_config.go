package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core/system"
	"github.com/kercylan98/wasteland/src/wasteland"
)

func newActorSystemConfig() *ActorSystemConfig {
	return &ActorSystemConfig{config: &system.Config{
		RPCMessageBuilder: wasteland.DefaultRPCMessageBuilder(),
	}}
}

type ActorSystemConfig struct {
	config *system.Config
}

// WithAddress 设置 ActorSystem 的网络地址
func (c *ActorSystemConfig) WithAddress(address string) *ActorSystemConfig {
	c.config.Address = address
	return c
}

// WithLoggerProvider 使用指定的日志提供者
func (c *ActorSystemConfig) WithLoggerProvider(provider log.Provider) *ActorSystemConfig {
	c.config.LoggerProvider = provider
	return c
}

// WithCodec 使用指定的编解码器对跨网络通信进行编解码
func (c *ActorSystemConfig) WithCodec(provider wasteland.CodecProvider, builder wasteland.RPCMessageBuilder) {
	c.config.CodecProvider = provider
	c.config.RPCMessageBuilder = builder
}
