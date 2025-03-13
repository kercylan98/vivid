package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

func newActorConfig() *ActorConfig {
	return &ActorConfig{config: &actor.Config{}}
}

type ActorConfig struct {
	config *actor.Config
}

func (c *ActorConfig) WithName(name string) *ActorConfig {
	c.config.Name = name
	return c
}
