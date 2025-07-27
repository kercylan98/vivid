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
- **异步任务支持**：支持任务附加和 Future 异步处理机制
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
    "fmt"
    "github.com/kercylan98/vivid/pkg/vivid"
)

func main() {
    // 创建 Actor 系统
    system := vivid.NewActorSystemWithOptions()
    defer system.Shutdown(true, "程序结束")

    // 创建一个简单的 Actor
    ref := system.SpawnOf(func() vivid.Actor {
        return vivid.ActorFN(func(ctx vivid.ActorContext) {
            // 处理不同类型的消息
            switch msg := ctx.Message().(type) {
            case *vivid.OnLaunch:
                fmt.Println("Actor 已启动")
            case string:
                fmt.Println("收到消息:", msg)
                if ctx.Sender() != nil {
                    ctx.Reply("已收到: " + msg)
                }
            }
        })
    })

    // 发送消息给 Actor
    system.Tell(ref, "你好，Actor！")

    // 发送消息并等待回复
    future := system.Ask(ref, "需要回复的消息")
    result, err := future.Result()
    if err != nil {
        fmt.Println("错误:", err)
    } else {
        fmt.Println("收到回复:", result)
    }
}
```

## 贡献指南

欢迎对 `Vivid` 项目做出贡献！您可以通过以下方式参与：

1. 提交 Bug 报告或功能请求
2. 提交代码改进或新功能的 Pull Request
3. 改进文档或添加示例
4. 分享使用经验和最佳实践
