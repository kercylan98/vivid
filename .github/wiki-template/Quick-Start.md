# 快速开始

本指南将帮助您快速上手 Vivid，包括安装、基本配置和简单示例。

## 安装

Vivid 需要 Go 1.24.0 或更高版本。使用以下命令安装：

```bash
go get github.com/kercylan98/vivid
```

## 创建一个简单的 Actor 系统

以下是一个最小化的 Vivid 应用程序：

```go
package main

import (
    "github.com/kercylan98/vivid/src/vivid"
    "fmt"
)

func main() {
    // 创建并启动 Actor 系统
    system := vivid.NewActorSystem().StartP()
    defer system.StopP()

    // 创建一个简单的 Actor
    ref := system.ActorOf(func() vivid.Actor {
        return vivid.ActorFN(func(ctx vivid.ActorContext) {
            // 处理不同类型的消息
            switch msg := ctx.Message().(type) {
            case *vivid.OnLaunch:
                fmt.Println("Actor 已启动")
            case string:
                fmt.Println("收到消息:", msg)
                ctx.Reply("已收到: " + msg)
            }
        })
    })

    // 发送消息给 Actor
    system.Tell(ref, "你好，Actor！")

    // 发送消息并等待回复
    result, err := system.Ask(ref, "需要回复的消息").Result()
    if err != nil {
        fmt.Println("错误:", err)
    } else {
        fmt.Println("收到回复:", result)
    }
}
```

## 消息传递模式

Vivid 支持多种消息传递模式：

### Tell（单向消息）

发送消息但不等待回复：

```go
system.Tell(actorRef, "单向消息")
```

### Ask（请求-响应）

发送消息并等待回复：

```go
future := system.Ask(actorRef, "需要回复的消息")
result, err := future.Result()
```

带超时的 Ask：

```go
future := system.Ask(actorRef, "需要回复的消息")
result, err := future.ResultWithin(5 * time.Second)
```

### Probe（可选回复）

发送消息，接收方可以选择回复：

```go
system.Probe(actorRef, "可选回复消息", func(reply interface{}, err error) {
    if err != nil {
        fmt.Println("错误:", err)
        return
    }
    fmt.Println("收到回复:", reply)
})
```

## 创建有状态的 Actor

Actor 可以封装状态：

```go
type CounterActor struct {
    count int
}

func (c *CounterActor) Receive(ctx vivid.ActorContext) {
    switch msg := ctx.Message().(type) {
    case *vivid.OnLaunch:
        fmt.Println("计数器 Actor 已启动")
    case string:
        if msg == "increment" {
            c.count++
            ctx.Reply(c.count)
        } else if msg == "get" {
            ctx.Reply(c.count)
        }
    }
}

// 创建 Actor
counterRef := system.ActorOf(func() vivid.Actor {
    return &CounterActor{count: 0}
})
```

## 下一步

- 查看[核心概念](Core-Concepts)了解更多关于 Actor 模型的信息
- 探索[高级功能](Advanced-Features)了解定时器、监督策略等功能
- 参考[配置选项](Configuration-Options)了解如何配置 Actor 系统