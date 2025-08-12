---
icon: gear
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

# ActorSystem 配置

在 Vivid 中，`ActorSystem` 提供了丰富的配置选项，以便开发者能够灵活的配置 `ActorSystem`。

> 所有的配置均会以 `With` 开头，并且返回 `ActorSystem` 实例，以便开发者能够链式调用。

## 日志记录器

默认情况下，`ActorSystem` 会使用内置的日志记录器来进行日志记录并输出到控制台。如果需要使用自定义的日志记录器，可以通过实现 [log.Logger](https://github.com/kercylan98/go-log) 提供的 `Logger` 接口来实现。

- `vivid.ActorSystemConfiguration.WithLogger()`
- `vivid.WithActorSystemLogger()`

## Future 默认超时时间

默认情况下，`ActorSystem` 会使用 `time.Duration` 记录默认超时时间，当使用 `vivid.ActorContext.Ask()` 或 `vivid.ActorContext.AskWithTimeout()` 时，如果未设置超时时间，则会使用该默认超时时间。

- `vivid.ActorSystemConfiguration.WithFutureDefaultTimeout()`
- `vivid.WithActorSystemFutureDefaultTimeoutFn()`

## 跨网络通信

网络通讯是 `ActorSystem` 的重要组成部分，它使得 `Actor` 能够在分布式环境中进行通信。

在 Vivid 中提供了透明化的跨网络通信，开发者无需关心网络通信的细节，只需要配置 `ActorSystem` 的网络配置即可。

> 当开发者的消息满足编解码器的要求时，Vivid 会自动将消息进行编解码，并进行网络通信。也就是说从本地消息到网络消息的转换是透明的，开发者无需关心。

- `vivid.ActorSystemConfiguration.WithNetwork()`
- `vivid.WithActorSystemNetwork()`

关于网络配置，也一样的使用了 `Configurator` 和 `Option` 的配置方式，开发者可以灵活的配置 `ActorSystem` 的网络配置。其配置项如下：

```go
type ActorSystemNetworkConfiguration struct {
	Network            string
	AdvertisedAddress  string
	BindAddress        string
	Server             processor.RPCServer
	Connector          processor.RPCConnProvider
	SerializerProvider provider.Provider[serializer.NameSerializer]
}
```

- `Network`：网络类型，默认使用 `tcp` 网络。
- `AdvertisedAddress`：广播地址，用于在网络中广播自己的地址。
- `BindAddress`：绑定地址，用于在网络中绑定自己的地址。
- `Server`：服务器，用于在网络中提供服务。
- `Connector`：连接器，用于在网络中连接其他节点。
- `SerializerProvider`：序列化器提供者，用于在网络中提供序列化器。


