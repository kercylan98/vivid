# 核心概念

本页面介绍 Vivid 中的核心概念和 Actor 模型的基本原理。

## Actor 模型简介

Actor 模型是一种并发计算模型，它将系统分解为独立的、封装的计算单元（Actor），这些单元通过消息传递进行通信。Actor 模型的核心原则包括：

1. **封装状态和行为**：每个 Actor 封装自己的状态和行为，外部无法直接访问
2. **消息传递通信**：Actor 之间只能通过异步消息传递进行通信
3. **位置透明性**：消息发送者不需要知道接收者的物理位置
4. **轻量级并发**：Actor 是轻量级的，一个系统可以包含数百万个 Actor

## Vivid 中的核心组件

### Actor

Actor 是系统中的基本计算单元。在 Vivid 中，Actor 是通过实现 `Actor` 接口或使用 `ActorFN` 函数类型来创建的：

```go
// Actor 接口
type Actor interface {
    Receive(ctx ActorContext)
}

// 使用 ActorFN 函数类型
type ActorFN func(ctx ActorContext)

func (fn ActorFN) Receive(ctx ActorContext) {
    fn(ctx)
}
```

### ActorSystem

ActorSystem 是 Actor 的容器和管理者，负责 Actor 的创建、监督和消息传递。一个应用程序通常只有一个 ActorSystem 实例：

```go
// 创建 ActorSystem
system := vivid.NewActorSystem()

// 启动 ActorSystem
system.Start()
// 或者使用 StartP() 方法（panic 版本）
system = system.StartP()

// 停止 ActorSystem
system.Stop()
// 或者使用 StopP() 方法（panic 版本）
system.StopP()
```

### ActorContext

ActorContext 提供了 Actor 与其环境交互的接口，包括：

- 访问当前消息
- 回复消息
- 创建子 Actor
- 访问 Actor 的路径和父 Actor
- 定时器功能
- 生命周期管理

```go
// ActorContext 接口的主要方法
type ActorContext interface {
    // 消息相关
    Message() interface{}
    Reply(reply interface{})
    
    // Actor 关系
    Self() ActorRef
    Parent() ActorRef
    Path() string
    
    // 子 Actor 创建
    ActorOf(factory ActorFactory) ActorRef
    ActorOfWithName(name string, factory ActorFactory) ActorRef
    
    // 定时器功能
    After(name string, duration time.Duration, fn func(time.Time))
    Loop(name string, delay, interval time.Duration, times int, fn func(time.Time))
    ForeverLoop(name string, delay, interval time.Duration, fn func(time.Time))
    Cron(name string, spec string, fn func(time.Time))
    
    // 生命周期管理
    Stop(target ActorRef)
}
```

### ActorRef

ActorRef 是 Actor 的引用，用于向 Actor 发送消息。ActorRef 隐藏了 Actor 的实现细节，提供了位置透明性：

```go
// 获取 ActorRef
ref := system.ActorOf(func() vivid.Actor {
    return &MyActor{}
})

// 使用 ActorRef 发送消息
system.Tell(ref, "消息")
```

## 消息处理

Actor 通过实现 `Receive` 方法来处理消息。通常使用类型断言来处理不同类型的消息：

```go
func (a *MyActor) Receive(ctx vivid.ActorContext) {
    switch msg := ctx.Message().(type) {
    case *vivid.OnLaunch:
        // 处理启动消息
    case string:
        // 处理字符串消息
    case int:
        // 处理整数消息
    default:
        // 处理未知类型的消息
    }
}
```

## 生命周期

Vivid 中的 Actor 有以下生命周期事件：

- **OnLaunch**：Actor 启动时
- **OnStop**：Actor 停止时
- **OnRestart**：Actor 重启时
- **OnError**：Actor 发生错误时

可以通过处理这些特殊消息来响应生命周期事件：

```go
func (a *MyActor) Receive(ctx vivid.ActorContext) {
    switch msg := ctx.Message().(type) {
    case *vivid.OnLaunch:
        fmt.Println("Actor 已启动")
    case *vivid.OnStop:
        fmt.Println("Actor 正在停止")
    case *vivid.OnRestart:
        fmt.Println("Actor 正在重启")
    case *vivid.OnError:
        fmt.Printf("Actor 发生错误: %v\n", msg.Error)
    }
}
```

## 监督策略

Vivid 提供了强大的监督机制，当 Actor 发生错误时，其监督者可以决定如何处理：

- **Restart**：重启 Actor
- **Stop**：停止 Actor
- **Resume**：继续执行，忽略错误

可以通过配置 Actor 来设置监督策略：

```go
ref := system.ActorOf(func() vivid.Actor {
    return &MyActor{}
}, func(config *vivid.ActorConfig) {
    config.WithSupervisor(func(ctx vivid.SupervisorContext) vivid.Directive {
        switch ctx.Error().(type) {
        case *SomeSpecificError:
            return vivid.Restart
        default:
            return vivid.Stop
        }
    })
})
```

## 下一步

- 查看[高级功能](Advanced-Features)了解定时器、远程通信等功能
- 参考[配置选项](Configuration-Options)了解如何配置 Actor 系统
- 阅读[API 参考](API-Reference)获取详细的 API 文档