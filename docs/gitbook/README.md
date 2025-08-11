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

# 📜 介绍

Vivid 是一款基于 Go 语言实现的​​高可扩展、协议无关的分布式系统开发框架​​，深度遵循 Actor 模型设计哲学，致力于为复杂分布式场景提供简洁、灵活且可靠的基础设施支持。框架通过函数式设计范式与模块化解耦架构，兼顾开发效率与运行时性能，适用于微服务、实时计算、高并发中间件等云原生场景。

在 Vivid 中，网络层不绑定任何特定通信协议，内部消息传输基于小端序（Little-Endian）实现高效二进制编解码，兼顾跨平台兼容性与空间效率；外部消息交互则开放编解码器扩展接口，开发者可根据业务需求灵活集成 Protobuf、JSON 或自定义协议。

关于消息投递，我们定义了 `Tell`/`Ask`/`Probe` 三态消息投递接口，以便能够覆盖分布式系统的多种通信场景：

* `Tell`：单向消息传递（无响应），适用于日志上报、事件广播等无需确认的场景；
* `Ask`：异步请求-响应模式，返回 future.Future对象，支持超时控制与非阻塞编程；
* `Probe`：带上下文的探测消息（携带发送者信息），适用于服务健康检查、链路追踪等需要元数据的场景。

> 消息投递完全兼容本地与远程调用。开发者无需修改业务代码，即可实现本地 Actor 与跨网络 Actor 的无缝交互。

Vivid 深度践行函数式编程范式，大量使用选项模式（Option Pattern）与配置器（Configurator）实现灵活的运行时配置。为复杂场景提供灵活的配置能力。

## 要求

Vivid 依赖于 Go 1.24+ 版本，支持 Linux、Windows、MacOS 平台。

## 安装

```bash
go get github.com/kercylan98/vivid
```


<!-- 
### Jump right in

<table data-view="cards"><thead><tr><th></th><th></th><th></th><th data-hidden data-card-cover data-type="files"></th><th data-hidden></th><th data-hidden data-card-target data-type="content-ref"></th></tr></thead><tbody><tr><td><h4><i class="fa-bolt">:bolt:</i></h4></td><td><strong>Quickstart</strong></td><td>Create your first site</td><td></td><td></td><td><a href="getting-started/quickstart.md">quickstart.md</a></td></tr><tr><td><h4><i class="fa-leaf">:leaf:</i></h4></td><td><strong>Editor basics</strong></td><td>Learn the basics of GitBook</td><td></td><td></td><td><a href="https://github.com/GitbookIO/gitbook-templates/blob/main/product-docs/broken-reference/README.md">https://github.com/GitbookIO/gitbook-templates/blob/main/product-docs/broken-reference/README.md</a></td></tr><tr><td><h4><i class="fa-globe-pointer">:globe-pointer:</i></h4></td><td><strong>Publish your docs</strong></td><td>Share your docs online</td><td></td><td></td><td><a href="getting-started/publish-your-docs.md">publish-your-docs.md</a></td></tr></tbody></table> -->
