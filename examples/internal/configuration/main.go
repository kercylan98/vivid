package main

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/vivid"
)

type MyConfigurator struct {
	Logger log.Logger
}

func (c *MyConfigurator) Configure(config *vivid.ActorSystemConfiguration) {
	config.WithLogger(c.Logger)
}

func main() {
	fromConfig()
	withFunctionalConfigurators()
	withStructConfigurators()
	withOptions()
}

func fromConfig() {
	config := vivid.NewActorSystemConfiguration(
		vivid.WithActorSystemLogger(log.GetDefault()),
	)

	vivid.NewActorSystemFromConfig(config)
}

func withFunctionalConfigurators() {
	vivid.NewActorSystemWithConfigurators(vivid.ActorSystemConfiguratorFN(func(config *vivid.ActorSystemConfiguration) {
		config.WithLogger(log.GetDefault())
	}))
}

func withStructConfigurators() {
	vivid.NewActorSystemWithConfigurators(&MyConfigurator{Logger: log.GetDefault()})
}

func withOptions() {
	vivid.NewActorSystemWithOptions(
		vivid.WithActorSystemLogger(log.GetDefault()),
	)
}
