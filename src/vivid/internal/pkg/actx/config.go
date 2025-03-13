package actx

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ actor.ConfigContext = (*Config)(nil)

func NewConfig() *Config {
	return &Config{}
}

type Config struct {
	Name string // Actor 名称
}

func (c *Config) GetName() string {
	return c.Name
}
