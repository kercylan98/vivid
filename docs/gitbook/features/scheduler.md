---
icon: clock
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

# 调度器

在 Vivid 中，为了方便的进行延迟任务的调度，我们为 `ActorSystem` 和 `ActorContext` 提供了调度接口，这些接口会自动将任务投递到对应的 Actor 中，并且支持取消任务。
    
当任务触发时，会将 `payload` 作为消息投递到 `targetRef` 对应的 Actor。

> 调度器是系统级的服务，由 `ActorSystem` 负责管理，因此调度器与 `Actor` 的交互是透明的，开发者无需关心调度器内部的实现。

## 自定义配置

当你需要调整 `ActorSystem` 的调度器配置时，可以通过 `ActorSystem` 的配置选项进行调整：

```go
sys := vivid.NewActorSystemWithOptions(vivid.WithActorSystemHTW(vivid.HtwConfig{
	Tick:      10 * time.Millisecond,
	WheelSize: 1024,
	Levels:    4,
}))
```

其中各参数的含义如下：
- `Tick`：调度器的时间精度，默认为 10 毫秒
- `WheelSize`：调度器的时间轮大小，默认为 256
- `Levels`：调度器的层级数量，默认为 4

当不需要调整调度器配置时，可以使用 `vivid.DefaultHtwConfig()` 获取默认配置：

```go
sys := vivid.NewActorSystemWithOptions(vivid.WithActorSystemHTW(vivid.DefaultHtwConfig()))
```

你也可以使用 `WithActorSystemHTW()` 配置选项进行配置：

```go
sys := vivid.NewActorSystemWithOptions(vivid.WithActorSystemHTW(vivid.HtwConfig{
	Tick:      10 * time.Millisecond,
	WheelSize: 1024,
	Levels:    4,
}))
```

## 一次性延迟任务

当需要使用 `vivid.ScheduleOnce()` 方法投递一次性延迟任务时，需要提供以下参数：
- `name`：任务名称，用于标识任务，相同名称的任务会覆盖
- `delay`：延迟时间，任务会在指定时间后触发
- `targetRef`：目标 Actor，任务会投递到目标 Actor 中
- `payload`：任务负载，任务会作为消息投递到目标 Actor 中

```go
context.ScheduleOnce("job-name", 500*time.Millisecond, targetRef, DoSomething{ID: 1}) // 500ms 后触发
```

## 周期性延迟任务

当需要使用 `vivid.ScheduleInterval()` 方法投递周期性延迟任务时，需要提供以下参数：
- `name`：任务名称，用于标识任务，相同名称的任务会覆盖
- `initialDelay`：初始延迟时间，任务会在指定时间后触发
- `period`：周期时间，任务会每隔指定时间触发一次
- `targetRef`：目标 Actor，任务会投递到目标 Actor 中
- `payload`：任务负载，任务会作为消息投递到目标 Actor 中

```go
context.ScheduleInterval("heartbeat", time.Second, 2*time.Second, targetRef, Ping{}) // 1s 后触发，每隔 2s 触发一次
```

## Cron 表达式任务

当需要使用 `vivid.ScheduleCron()` 方法投递 Cron 表达式任务时，需要提供以下参数：
- `name`：任务名称，用于标识任务，相同名称的任务会覆盖
- `spec`：Cron 表达式，任务会按照指定表达式触发
- `targetRef`：目标 Actor，任务会投递到目标 Actor 中
- `payload`：任务负载，任务会作为消息投递到目标 Actor 中

```go
context.ScheduleCron("report", "0 0 * * *", targetRef, GenerateReport{}) // 每天 0 点触发
```

## 取消任务

当需要使用 `vivid.CancelSchedule()` 方法取消任务时，需要提供任务名称：

```go
context.CancelSchedule("heartbeat")
```

取消任务后，任务将立即从调度器中移除，不会触发并销毁任何任务。即便是该任务已经触发，但是由于 Actor 消息阻塞导致在执行前取消，调度器也会立即取消任务。

## 注意事项

任务的执行可能受到消息阻塞的影响，如：`vivid.Tell()` 方法投递的消息会在当前消息处理完成后投递，因此如果消息处理时间过长，任务可能会延迟触发。

