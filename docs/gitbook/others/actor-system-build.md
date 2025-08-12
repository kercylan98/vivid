---
icon: gear
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

# ActorSystem 构建

在 Vivid 中，`ActorSystem` 是整个系统的核心，它负责管理所有的 `Actor` 实例，并且提供了一系列的配置选项，以便开发者能够灵活的配置 `ActorSystem`。

## 通过 Configurator 构建

在 Vivid 中，你可以使用 `vivid.ActorSystemConfigurator` 构建 `ActorSystem`。

它接受一个接口类型，同时也提供函数式的配置选项。

### 接口式配置

在接口式配置中，你需要实现 `vivid.ActorSystemConfigurator` 接口，例如：

```go
type MyConfigurator struct{}

func (c *MyConfigurator) Configure(config *vivid.ActorSystemConfiguration) {
	config.WithLogger(log.GetDefault())
}
```

然后使用 `vivid.NewActorSystemWithConfigurators()` 方法创建 `ActorSystem` 实例：

```go
sys := vivid.NewActorSystemWithConfigurators(MyConfigurator{})
```

> 这也是我们推荐的方式，因为它足够的灵活且便捷。为此我们默认的 `vivid.NewActorSystem()` 方法就是通过 `vivid.NewActorSystemWithConfigurators()` 方法创建的。

### 函数式配置

在函数式配置中，无需实现 `vivid.ActorSystemConfigurator` 接口，直接使用 `vivid.ActorSystemConfiguratorFN` 便可快速创建 `ActorSystem` 实例，它本质上是一个实现了 `vivid.ActorSystemConfigurator` 接口的函数类型。

```go
sys := vivid.NewActorSystemWithConfigurators(vivid.ActorSystemConfiguratorFN(func(config *vivid.ActorSystemConfiguration) {
	config.WithLogger(log.GetDefault())
}))
```

## 通过 Options 构建

使用 `vivid.NewActorSystemWithOptions()` 方法创建 `ActorSystem` 实例，它接受一个 `vivid.ActorSystemOption` 类型的参数，你可以通过该参数配置 `ActorSystem` 的各个选项。

```go
sys := vivid.NewActorSystemWithOptions(vivid.WithActorSystemLogger(log.GetDefault()))
```

## 通过 Configuration 构建

使用 `vivid.NewActorSystemFromConfiguration()` 方法创建 `ActorSystem` 实例，它接受一个 `vivid.ActorSystemConfiguration` 类型的参数，你可以通过该参数配置 `ActorSystem` 的各个选项。

```go
sys := vivid.NewActorSystemFromConfiguration(vivid.NewActorSystemConfiguration())
```

在 `vivid.NewActorSystemConfiguration()` 方法中，它支持通过 Options 的方式配置 `ActorSystem` 的各个选项。

```go
config := vivid.NewActorSystemConfiguration(vivid.WithActorSystemLogger(log.GetDefault()))
sys := vivid.NewActorSystemFromConfiguration(config)
```
