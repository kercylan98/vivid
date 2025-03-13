package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/system"

func newActorSystemConfig() *ActorSystemConfig {
	return &ActorSystemConfig{config: &system.Config{}}
}

type ActorSystemConfig struct {
	config *system.Config
}

func (c *ActorSystemConfig) WithAddress(address string) *ActorSystemConfig {
	c.config.Address = address
	return c
}
