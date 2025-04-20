// metrics_plugin.go 演示如何创建一个 Vivid 插件

package main

import (
    "fmt"
    "github.com/kercylan98/vivid/src/vivid"
    "sync/atomic"
    "time"
)

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

// MetricsPlugin 是一个简单的指标收集插件，用于演示 Vivid 的插件系统，
// 它收集 Actor 系统中的消息数量和处理时间等指标。
type MetricsPlugin struct {
    vivid.BasePlugin
    messageCount     atomic.Int64
    processingTimeNs atomic.Int64
    system           vivid.ActorSystem
    metricsActor     vivid.ActorRef
    stopChan         chan struct{}
}

// Initialize 初始化插件
func (p *MetricsPlugin) Initialize(system vivid.ActorSystem) error {
    p.system = system

    // 创建一个 Actor 来收集和处理指标
    p.metricsActor = system.ActorOf(func() vivid.Actor {
        return vivid.ActorFN(func(ctx vivid.ActorContext) {
            switch msg := ctx.Message().(type) {
            case *vivid.OnLaunch:
                fmt.Println("指标收集器已启动")
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

    // 启动一个后台 goroutine 定期打印指标
    go p.reportMetrics()

    fmt.Printf("指标收集插件已初始化，ID: %s, 版本: %s\n", p.ID(), p.Version())
    return nil
}

// Shutdown 关闭插件
func (p *MetricsPlugin) Shutdown() error {
    close(p.stopChan)
    fmt.Println("指标收集插件已关闭")
    return nil
}

// reportMetrics 定期打印指标
func (p *MetricsPlugin) reportMetrics() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            metrics, err := p.GetMetrics()
            if err != nil {
                fmt.Printf("获取指标失败: %v\n", err)
                continue
            }

            fmt.Printf("系统指标 - 消息数: %d, 平均处理时间: %.2f ns\n",
                metrics.MessageCount,
                float64(metrics.ProcessingTimeNs)/float64(metrics.MessageCount+1),
            )

        case <-p.stopChan:
            return
        }
    }
}

// RecordMessage 记录一条消息的处理时间
func (p *MetricsPlugin) RecordMessage(processingTime time.Duration) {
    p.system.Tell(p.metricsActor, &MessageProcessed{
        ProcessingTime: processingTime,
    })
}

// GetMetrics 获取当前指标
func (p *MetricsPlugin) GetMetrics() (*Metrics, error) {
    future := p.system.Ask(p.metricsActor, &GetMetrics{})
    result, err := future.Result()
    if err != nil {
        return nil, err
    }

    metrics, ok := result.(*Metrics)
    if !ok {
        return nil, fmt.Errorf("unexpected result type: %T", result)
    }

    return metrics, nil
}

// MessageProcessed 表示一条消息已处理
type MessageProcessed struct {
    ProcessingTime time.Duration
}

// GetMetrics 请求获取当前指标
type GetMetrics struct{}

// Metrics 包含系统指标
type Metrics struct {
    MessageCount     int64
    ProcessingTimeNs int64
    Timestamp        time.Time
}

// 使用示例
func main() {
    // 创建插件
    metricsPlugin := NewMetricsPlugin()

    // 创建 Actor 系统并注册插件
    system := vivid.NewActorSystem()
    err := system.RegisterPlugin(metricsPlugin)
    if err != nil {
        panic(err)
    }

    // 启动系统
    system.StartP()
    defer system.StopP()

    // 创建一个示例 Actor
    exampleActor := system.ActorOf(func() vivid.Actor {
        return vivid.ActorFN(func(ctx vivid.ActorContext) {
            start := time.Now()

            switch msg := ctx.Message().(type) {
            case string:
                // 模拟处理时间
                time.Sleep(10 * time.Millisecond)
                fmt.Printf("处理消息: %s\n", msg)
            }

            // 记录处理时间
            metricsPlugin.RecordMessage(time.Since(start))
        })
    })

    // 发送一些消息
    for i := 0; i < 10; i++ {
        system.Tell(exampleActor, fmt.Sprintf("消息 %d", i))
        time.Sleep(500 * time.Millisecond)
    }

    // 获取并打印指标
    metrics, err := metricsPlugin.GetMetrics()
    if err != nil {
        fmt.Printf("获取指标失败: %v\n", err)
    } else {
        fmt.Printf("最终指标 - 消息数: %d, 总处理时间: %d ns\n",
            metrics.MessageCount,
            metrics.ProcessingTimeNs,
        )
    }

    // 等待一段时间以便查看定期报告
    time.Sleep(5 * time.Second)
}
