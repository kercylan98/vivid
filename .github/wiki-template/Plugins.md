# 插件系统

Vivid 提供了一个灵活的插件系统，允许您扩展 Actor 系统的功能。插件可以用于添加监控、日志记录、性能分析等功能，而无需修改核心代码。

## 插件接口

所有插件必须实现 `Plugin` 接口：

```go
type Plugin interface {
    // ID 返回插件的唯一标识符
    ID() string

    // Name 返回插件的名称
    Name() string

    // Version 返回插件的版本
    Version() string

    // Description 返回插件的描述
    Description() string

    // Initialize 在 ActorSystem 启动时被调用，用于初始化插件
    // 如果返回错误，ActorSystem 将不会加载该插件
    Initialize(system ActorSystem) error

    // Shutdown 在 ActorSystem 关闭时被调用，用于清理资源
    Shutdown() error
}
```

## 创建插件

创建插件的最简单方法是嵌入 `BasePlugin` 类型，它提供了 `Plugin` 接口的基本实现：

```go
type MyPlugin struct {
    vivid.BasePlugin
    // 插件特定的字段
}

func NewMyPlugin() *MyPlugin {
    return &MyPlugin{
        BasePlugin: vivid.NewBasePlugin(
            "my-plugin",           // ID
            "My Plugin",           // 名称
            "1.0.0",               // 版本
            "这是我的自定义插件",    // 描述
        ),
        // 初始化插件特定的字段
    }
}

// 重写 Initialize 方法
func (p *MyPlugin) Initialize(system vivid.ActorSystem) error {
    // 执行插件初始化
    return nil
}

// 重写 Shutdown 方法
func (p *MyPlugin) Shutdown() error {
    // 执行插件清理
    return nil
}
```

## 注册和使用插件

要使用插件，您需要在启动 ActorSystem 之前注册它：

```go
// 创建插件
myPlugin := NewMyPlugin()

// 创建 Actor 系统
system := vivid.NewActorSystem()

// 注册插件
err := system.RegisterPlugin(myPlugin)
if err != nil {
    panic(err)
}

// 启动系统
system.StartP()
defer system.StopP()
```

## 插件生命周期

1. **注册**：通过 `RegisterPlugin` 方法注册插件
2. **初始化**：当 ActorSystem 启动时，调用所有已注册插件的 `Initialize` 方法
3. **运行**：插件在 ActorSystem 运行期间执行其功能
4. **关闭**：当 ActorSystem 停止时，调用所有插件的 `Shutdown` 方法

## 插件示例

### 指标收集插件

以下是一个指标收集插件的示例，它记录消息数量和处理时间：

```go
// MetricsPlugin 是一个简单的指标收集插件
type MetricsPlugin struct {
    vivid.BasePlugin
    messageCount     atomic.Int64
    processingTimeNs atomic.Int64
    system           vivid.ActorSystem
    metricsActor     vivid.ActorRef
    stopChan         chan struct{}
}

// NewMetricsPlugin 创建一个新的指标收集插件
func NewMetricsPlugin() *MetricsPlugin {
    return &MetricsPlugin{
        BasePlugin: vivid.NewBasePlugin(
            "metrics",
            "Metrics Plugin",
            "1.0.0",
            "收集 Actor 系统中的消息数量和处理时间等指标",
        ),
        stopChan: make(chan struct{}),
    }
}

// Initialize 初始化插件
func (p *MetricsPlugin) Initialize(system vivid.ActorSystem) error {
    p.system = system
    
    // 创建一个 Actor 来收集和处理指标
    p.metricsActor = system.ActorOf(func() vivid.Actor {
        return vivid.ActorFN(func(ctx vivid.ActorContext) {
            switch msg := ctx.Message().(type) {
            case *MessageProcessed:
                p.messageCount.Add(1)
                p.processingTimeNs.Add(int64(msg.ProcessingTime))
            case *GetMetrics:
                ctx.Reply(&Metrics{
                    MessageCount:     p.messageCount.Load(),
                    ProcessingTimeNs: p.processingTimeNs.Load(),
                    Timestamp:        time.Now(),
                })
            }
        })
    })
    
    return nil
}

// Shutdown 关闭插件
func (p *MetricsPlugin) Shutdown() error {
    close(p.stopChan)
    return nil
}

// RecordMessage 记录一条消息的处理时间
func (p *MetricsPlugin) RecordMessage(processingTime time.Duration) {
    p.system.Tell(p.metricsActor, &MessageProcessed{
        ProcessingTime: processingTime,
    })
}
```

完整的示例代码可以在 [examples/plugins/metrics_plugin.go](https://github.com/kercylan98/vivid/blob/main/src/vivid/examples/plugins/metrics_plugin.go) 中找到。

## 最佳实践

1. **唯一 ID**：确保插件 ID 是唯一的，以避免冲突
2. **错误处理**：在 Initialize 和 Shutdown 方法中正确处理错误
3. **资源清理**：在 Shutdown 方法中清理所有资源，如 goroutines、文件句柄等
4. **线程安全**：确保插件是线程安全的，因为它可能会被多个 goroutines 同时访问
5. **文档**：为插件提供清晰的文档，包括其功能、配置选项和使用示例