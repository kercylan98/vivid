# Vivid

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/kercylan98/vivid)](https://github.com/kercylan98/vivid)
[![Last Commit](https://img.shields.io/github/last-commit/kercylan98/vivid)](https://github.com/kercylan98/vivid/commits/main)
[![Open Issues](https://img.shields.io/github/issues/kercylan98/vivid)](https://github.com/kercylan98/vivid/issues)
[![Go Report Card](https://goreportcard.com/badge/github.com/kercylan98/vivid)](https://goreportcard.com/report/github.com/kercylan98/vivid)
[![Stars](https://img.shields.io/github/stars/kercylan98/vivid)](https://github.com/kercylan98/vivid/stargazers)

> **注意**: 该项目目前处于积极开发阶段，API 可能会发生变化。

Vivid 是一个为分布式系统设计的完整 Actor 模型实现，源于 `Minotaur` 项目的功能拆解重构。

## 项目简介

Vivid 提供了一个强大、灵活且高性能的 Actor 模型框架，专为构建可扩展的并发和分布式系统而设计。Actor 模型是一种并发计算模型，它将系统分解为独立的、封装的计算单元（Actor），这些单元通过消息传递进行通信，从而简化了并发系统的设计和实现。

## 特性

- **高性能消息传递**：优化的消息队列和调度机制，确保高效的 Actor 间通信
- **灵活的 Actor 生命周期管理**：完整支持 Actor 的创建、监督和终止
- **强大的监督策略**：内置故障恢复机制，支持自定义监督策略
- **定时任务支持**：内置定时器功能，支持延迟执行、循环执行和 Cron 表达式
- **分布式通信**：支持跨网络的 Actor 通信，适用于分布式系统
- **多种消息模式**：支持 Tell（单向消息）、Probe（可选回复）和 Ask（等待回复）等多种消息传递模式
- **函数式 API**：提供简洁的函数式 API，简化 Actor 的创建和使用
- **可配置的日志系统**：灵活的日志配置，满足不同的调试和监控需求

## 安装

Vivid 需要 Go 1.24.0 或更高版本。使用以下命令安装：

```bash
go get github.com/kercylan98/vivid
```

## 快速开始

以下是一个简单的示例，展示如何创建和使用 Actor：

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

## 核心概念

### Actor

Actor 是系统中的基本计算单元，它封装了状态和行为，并通过消息传递与其他 Actor 通信。在 Vivid 中，Actor 是通过实现 `Actor` 接口或使用 `ActorFN` 函数类型来创建的。

### ActorSystem

ActorSystem 是 Actor 的容器和管理者，负责 Actor 的创建、监督和消息传递。一个应用程序通常只有一个 ActorSystem 实例。

### ActorContext

ActorContext 提供了 Actor 与其环境交互的接口，包括发送消息、回复消息、创建子 Actor 等功能。

### ActorRef

ActorRef 是 Actor 的引用，用于向 Actor 发送消息。ActorRef 隐藏了 Actor 的实现细节，提供了位置透明性。

### 消息传递

Vivid 支持多种消息传递模式：

- **Tell**：发送单向消息，不等待回复
- **Probe**：发送消息，接收方可以选择回复
- **Ask**：发送消息并等待回复，支持超时设置

### 监督策略

Vivid 提供了强大的监督机制，当 Actor 发生错误时，其监督者可以决定如何处理：重启、停止或继续。

## 高级功能

### 定时任务

Vivid 内置了定时器功能，支持：

- **After**：延迟执行任务
- **Loop**：循环执行任务
- **ForeverLoop**：无限循环执行任务
- **Cron**：使用 Cron 表达式调度任务

```go
// 延迟执行
ctx.After("task1", 5*time.Second, func(t time.Time) {
    fmt.Println("5秒后执行")
})

// 循环执行
ctx.Loop("task2", 0, 1*time.Second, 5, func(t time.Time) {
    fmt.Println("每秒执行一次，共执行5次")
})

// Cron 表达式
ctx.Cron("task3", "0 */5 * * * *", func(t time.Time) {
    fmt.Println("每5分钟执行一次")
})
```

### 分布式通信

Vivid 支持跨网络的 Actor 通信，可以通过配置 ActorSystem 的网络地址来启用：

```go
system := vivid.NewActorSystem(func(config *vivid.ActorSystemConfig) {
    config.WithAddress("127.0.0.1:8080")
})
```

## 配置选项

Vivid 提供了多种配置选项，可以通过 `ActorSystemConfig` 进行设置：

- **WithAddress**：设置网络地址
- **WithLoggerProvider**：设置日志提供者
- **WithCodec**：设置网络通信编解码器
- **WithGuardDefaultRestartLimit**：设置默认重启限制
- **WithTimingWheelTick**：设置定时器滴答时间
- **WithTimingWheelSize**：设置定时器大小

## 架构概览

Vivid 的架构由以下主要组件组成：

1. **核心层**：包含 Actor 模型的基本抽象和接口
2. **实现层**：提供核心接口的具体实现
3. **用户 API**：简化的用户接口，隐藏内部复杂性
4. **扩展模块**：提供额外功能，如定时器、监督策略等

## API 文档

详细的 API 文档可以通过 Go 的标准文档工具生成：

```bash
go doc github.com/kercylan98/vivid/src/vivid
```

## 贡献指南

欢迎对 Vivid 项目做出贡献！您可以通过以下方式参与：

1. 提交 Bug 报告或功能请求
2. 提交代码改进或新功能的 Pull Request
3. 改进文档或添加示例
4. 分享使用经验和最佳实践

## 许可证

Vivid 使用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

```
MIT License

Copyright (c) 2023 kercylan98

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

# JetBrains OS licenses

`Vivid` had been being developed with `GoLand` IDE under the **free JetBrains Open Source license(s)** granted by JetBrains s.r.o., hence I would like to express my thanks here.

<a href="https://www.jetbrains.com/?from=minotaur" target="_blank"><img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jb_beam.png?_gl=1*1vt713y*_ga*MTEzMjEzODQxNC4xNjc5OTY3ODUw*_ga_9J976DJZ68*MTY4ODU0MDUyMy4yMC4xLjE2ODg1NDA5NDAuMjUuMC4w&_ga=2.261225293.1519421387.1688540524-1132138414.1679967850" width="250" align="middle"/></a>
