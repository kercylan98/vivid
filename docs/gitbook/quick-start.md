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

# 🔧 快速开始

使用 Vivid 开发分布式系统，是极其简单的，你只需要创建一个 `ActorSystem` 实例，然后设计一个 `Actor` 并将其使用 `ActorSystem` 启动即可。

## 创建 ActorSystem

关于 `ActorSystem` 的创建，你可以使用 `vivid.NewActorSystem()` 方法创建一个默认配置的 `ActorSystem` 实例。如果需要更灵活的创建方式或自定义配置，可以参考 [ActorSystem 构建](others/actor-system-build.md) 和 [ActorSystem 配置](others/actor-system-config.md) 来获取更多信息。

```go
sys := vivid.NewActorSystem()
```

## 创建 Actor

```go