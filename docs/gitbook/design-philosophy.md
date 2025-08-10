---
layout:
  width: default
  title:
    visible: true
  description:
    visible: true
  tableOfContents:
    visible: true
  outline:
    visible: true
  pagination:
    visible: true
  metadata:
    visible: true
---

# 🏦 设计哲学

Vivid 皆在于提供一个健壮且高效，简洁又灵活的分布式系统开发框架。遵循​​最小抽象原则​​与​​显式优于隐式​​的理念以降低开发者心智负担。

## 透明的配置体系

在 Vivid 中，所有配置均通过选项模式（Option Pattern）与配置器（Configurator）实现，开发者可以轻松的进行配置，并且支持对复杂场景的配置。

```go
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
```

## 函数式行为的轻量表达

Vivid 中针对大量内容提供了函数式行为的轻量表达，例如通过简单的函数式风格即可创建 Actor 并启动。

```go
package main

import (
	"fmt"

	"github.com/kercylan98/vivid/pkg/vivid"
)

func main() {
	var wait = make(chan struct{})
	system := vivid.NewActorSystem()
	system.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case *vivid.OnLaunch:
				fmt.Println("hello")
				close(wait)
			}
		})
	})

	<-wait

	system.Shutdown(true)
}
```

当然，函数式风格的表达能力远不止于此，其中大量的提供器（Provider）、配置器（Configurator）等均提供了函数式风格的轻量表达。
