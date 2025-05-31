package vivid

import (
    "fmt"
    "strings"
    "sync"
    "time"

    "github.com/kercylan98/vivid/src/persistence"
    "github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

var _ actor.Actor = (*actorFacade)(nil)

// Actor 是系统中的基本计算单元接口，它封装了状态和行为，并通过消息传递与其他 Actor 通信。
//
// 所有 Actor 必须实现 OnReceive 方法来处理接收到的消息。
//
// 如果所需的 Actor 功能简单，可以使用 ActorFN 函数类型来简化实现。
type Actor interface {
    // OnReceive 当 Actor 接收到消息时被调用。
    //
    // 参数 ctx 提供了访问当前消息上下文的能力，包括获取消息内容、发送者信息以及回复消息等功能。
    OnReceive(ctx ActorContext)
}

// ActorFN 是一个函数类型，实现了 Actor 接口。
//
// 它允许使用函数式编程风格来创建 Actor，简化了 Actor 的定义过程。
type ActorFN func(ctx ActorContext)

// OnReceive 实现 Actor 接口的 OnReceive 方法。
//
// 它直接调用函数本身，将上下文传递给函数处理。
func (fn ActorFN) OnReceive(ctx ActorContext) {
    fn(ctx)
}

// ActorProvider 是 Actor 提供者接口。
//
// 它负责创建和提供 Actor 实例，用于延迟 Actor 的实例化。
//
// 如果 ActorProvider 的实现简单，可以使用 ActorProviderFN 函数类型来简化实现。
type ActorProvider interface {
    // Provide 返回一个新的 Actor 实例
    Provide() Actor
}

// ActorProviderFN 是一个函数类型，实现了 ActorProvider 接口
//
// 它允许使用函数式编程风格来创建 ActorProvider，简化了 ActorProvider 的定义过程
type ActorProviderFN func() Actor

// Provide 实现 ActorProvider 接口的 Provide 方法。
//
// 它直接调用函数本身，返回一个新的 Actor 实例
func (fn ActorProviderFN) Provide() Actor {
    return fn()
}

// newActorFacade 创建一个 Actor 门面代理，
// 该函数接收系统实例、父上下文、Actor提供者和配置参数，返回创建的 Actor 引用，
// 门面代理模式用于在 Actor 的外部和内部之间提供一个统一的接口，处理消息转换和生命周期事件。
func newActorFacade(system actor.System, parent actor.Context, provider ActorProvider, configuration ...ActorConfigurator) ActorRef {
    config := newActorConfig()
    for _, c := range configuration {
        c.Configure(config)
    }

    // 预先检查是否为持久化 Actor，并准备持久化状态
    actorInstance := provider.Provide()
    var persistenceState *persistence.State

    // 检查持久化Actor
    if persistentActor, ok := actorInstance.(PersistentActor); ok && config.repository != nil {
        persistenceId := persistentActor.GetPersistenceId()
        if persistenceId != "" {
            persistenceState = persistence.NewState(persistenceId, config.repository)
        }
    }

    // 创建 Actor 门面代理的提供器，确保每次生成均能够获得全新的 Actor 实例
    var facadeCtx ActorContext
    // 由于是异步，所以需要等待 facadeCtx 的创建
    var waiter sync.WaitGroup
    waiter.Add(1)
    facadeProvider := actor.ProviderFN(func() actor.Actor {
        // 创建 Actor 门面代理，重用同一个Actor实例
        facade := &actorFacade{actor: actorInstance}

        // 如果支持持久化
        if persistentActor, ok := actorInstance.(PersistentActor); ok && persistenceState != nil {
            facade.persistentActor = persistentActor
            facade.persistenceState = persistenceState
        }

        // 设置 Actor 门面代理的 Actor 方法
        facade.Actor = actor.FN(func(ctx actor.Context) {
            startTime := time.Now() // 记录消息处理开始时间

            // 内部消息类型转换，处理系统消息和用户消息
            switch msg := ctx.MessageContext().Message().(type) {
            case *actor.OnLaunch:
                waiter.Wait()

                // 记录系统消息接收到监控系统
                defer func() {
                    if monitoringCtx := facadeCtx.Monitoring(); monitoringCtx.IsEnabled() {
                        latency := time.Since(startTime)
                        // 使用系统消息记录方法
                        if m, ok := monitoringCtx.(*monitoringContext); ok && m.metrics != nil {
                            if im, ok := m.metrics.(internalMetrics); ok && im.IsRecording() {
                                im.recordSystemMessageReceived(m.actorRef, "OnLaunch", latency)
                            }
                        }
                    }
                }()

                // 持久化恢复
                if facade.persistentActor != nil && facade.persistenceState != nil {
                    _ = autoRecover(facade.persistenceState, facade.persistentActor)
                }

                // 执行用户消息处理
                func() {
                    defer func() {
                        if r := recover(); r != nil {
                            // 记录错误到监控系统
                            if monitoringCtx := facadeCtx.Monitoring(); monitoringCtx.IsEnabled() {
                                monitoringCtx.RecordError(fmt.Errorf("actor panic: %v", r), "OnLaunch")
                            }
                            panic(r) // 重新抛出panic
                        }
                    }()

                    facade.actor.OnReceive(facadeCtx)
                }()

            case *actor.OnKill:
                // 记录系统消息接收到监控系统
                if monitoringCtx := facadeCtx.Monitoring(); monitoringCtx.IsEnabled() {
                    latency := time.Since(startTime)
                    if m, ok := monitoringCtx.(*monitoringContext); ok && m.metrics != nil {
                        if im, ok := m.metrics.(internalMetrics); ok && im.IsRecording() {
                            im.recordSystemMessageReceived(m.actorRef, "OnKill", latency)
                        }
                    }
                }

                // 在 Actor 被杀死前，自动保存持久化状态
                if facade.persistentActor != nil && facade.persistenceState != nil && config.snapshotPolicy != nil && config.snapshotPolicy.ForceSnapshotOnShutdown {
                    if persistenceCtx := facadeCtx.Persistence(); persistenceCtx != nil {
                        _ = persistenceCtx.ForceSnapshot()
                    }
                }

                // 注意：这里HandleWith会触发内部OnKill处理，但不是网络消息
                // 内部消息处理不应该计入发送统计
                // 这个HandleWith会重新调用OnReceive，但应该跳过监控
                ctx.MessageContext().HandleWith(&OnKill{m: msg})

            case *actor.OnKilled:
                // OnKilled是内部生命周期消息，通过HandleWith注入，不是真正的消息传递
                // 不记录到监控系统，避免接收计数不匹配
                // 用户无需处理，屏蔽该消息

            case *actor.OnDead:
                // 记录系统消息接收到监控系统
                if monitoringCtx := facadeCtx.Monitoring(); monitoringCtx.IsEnabled() {
                    latency := time.Since(startTime)
                    if m, ok := monitoringCtx.(*monitoringContext); ok && m.metrics != nil {
                        if im, ok := m.metrics.(internalMetrics); ok && im.IsRecording() {
                            im.recordSystemMessageReceived(m.actorRef, "OnDead", latency)
                        }
                    }
                }

                // 注意：这里HandleWith会触发内部OnDead处理
                // 这个HandleWith会重新调用OnReceive，但应该跳过监控
                ctx.MessageContext().HandleWith(&OnDead{m: msg})
            default:
                // 处理其他消息
                func() {
                    defer func() {
                        if r := recover(); r != nil {
                            // 记录错误到监控系统
                            if monitoringCtx := facadeCtx.Monitoring(); monitoringCtx.IsEnabled() {
                                monitoringCtx.RecordError(fmt.Errorf("actor panic: %v", r), "message_handling")
                            }
                            panic(r) // 重新抛出panic
                        }

                        // 记录用户消息接收到监控系统
                        if monitoringCtx := facadeCtx.Monitoring(); monitoringCtx.IsEnabled() {
                            latency := time.Since(startTime)
                            messageType := getMessageTypeName(msg)
                            if m, ok := monitoringCtx.(*monitoringContext); ok && m.metrics != nil {
                                if im, ok := m.metrics.(internalMetrics); ok && im.IsRecording() {
                                    im.recordUserMessageReceived(m.actorRef, messageType, latency)
                                } else {
                                    // 兼容旧方法
                                    monitoringCtx.RecordMessageReceived(messageType, latency)
                                }
                            }
                        }
                    }()

                    facade.actor.OnReceive(facadeCtx)
                }()

                // 注意：不在这里自动保存持久化状态
                // 持久化应该由用户在消息处理逻辑中主动调用
            }
        })
        return facade
    })

    // 创建上下文完毕后写入 waiter，通知 Actor 初始化完成（避免竞态问题）
    internalCtx := parent.GenerateContext().GenerateActorContext(system, parent, facadeProvider, *config.config)

    // 将监控配置传递到内部配置
    if config.monitoring != nil {
        // 创建适配器，将外部监控接口适配为内部接口
        if internalMetrics, ok := config.monitoring.(internalMetrics); ok {
            internalCtx.MetadataContext().Config().Monitoring = &monitoringAdapter{
                metrics: internalMetrics,
            }
        }
    }

    // 根据配置类型创建上下文
    var monitoringCtx MonitoringContext
    if config.monitoring != nil {
        monitoringCtx = newMonitoringContext(config.monitoring, internalCtx.MetadataContext().Ref())
    }

    if persistenceState != nil {
        // 需要从 actorInstance 转换为 PersistentActor
        if persistentActor, ok := actorInstance.(PersistentActor); ok {
            persistenceCtx := newPersistenceContext(persistenceState, config.snapshotPolicy, persistentActor)
            if monitoringCtx != nil {
                facadeCtx = newActorContextWithPersistenceAndMonitoring(internalCtx, persistenceCtx, monitoringCtx)
            } else {
                facadeCtx = newActorContextWithPersistence(internalCtx, persistenceCtx)
            }
        }
    } else {
        if monitoringCtx != nil {
            facadeCtx = newActorContextWithMonitoring(internalCtx, monitoringCtx)
        } else {
            facadeCtx = newActorContext(internalCtx)
        }
    }

    waiter.Done()
    return facadeCtx.Ref()
}

// getMessageTypeName 获取消息类型名称，用于监控记录
func getMessageTypeName(message interface{}) string {
    if message == nil {
        return "nil"
    }

    // 处理指针类型
    typeName := fmt.Sprintf("%T", message)
    if strings.HasPrefix(typeName, "*") {
        typeName = typeName[1:] // 移除前导的*
    }

    // 移除包名，只保留类型名
    if lastDot := strings.LastIndex(typeName, "."); lastDot != -1 {
        typeName = typeName[lastDot+1:]
    }

    return typeName
}

// monitoringAdapter 适配器，将外部监控接口适配为内部接口
type monitoringAdapter struct {
    metrics internalMetrics // 使用内部接口
}

func (m *monitoringAdapter) RecordMessageSent(from, to actor.Ref, messageType string) {
    if m.metrics != nil && m.metrics.IsRecording() {
        m.metrics.RecordMessageSent(from, to, messageType)
    }
}

func (m *monitoringAdapter) RecordUserMessageSent(from, to actor.Ref, messageType string) {
    if m.metrics != nil && m.metrics.IsRecording() {
        m.metrics.recordUserMessageSent(from, to, messageType)
    }
}

func (m *monitoringAdapter) RecordSystemMessageSent(from, to actor.Ref, messageType string) {
    if m.metrics != nil && m.metrics.IsRecording() {
        m.metrics.recordSystemMessageSent(from, to, messageType)
    }
}

func (m *monitoringAdapter) RecordUserMessageReceived(actor actor.Ref, messageType string, latency int64) {
    if m.metrics != nil && m.metrics.IsRecording() {
        m.metrics.recordUserMessageReceived(actor, messageType, time.Duration(latency))
    }
}

func (m *monitoringAdapter) RecordSystemMessageReceived(actor actor.Ref, messageType string, latency int64) {
    if m.metrics != nil && m.metrics.IsRecording() {
        m.metrics.recordSystemMessageReceived(actor, messageType, time.Duration(latency))
    }
}

// globalMonitoringAdapter 全局监控适配器，用于系统级Actor的监控
type globalMonitoringAdapter struct {
    metrics Metrics
}

func (g *globalMonitoringAdapter) RecordMessageSent(from, to actor.Ref, messageType string) {
    if g.metrics != nil {
        // 尝试转换为内部接口以访问完整功能
        if im, ok := g.metrics.(internalMetrics); ok && im.IsRecording() {
            im.RecordMessageSent(from, to, messageType)
        }
    }
}

func (g *globalMonitoringAdapter) RecordUserMessageSent(from, to actor.Ref, messageType string) {
    if g.metrics != nil {
        if im, ok := g.metrics.(internalMetrics); ok && im.IsRecording() {
            im.recordUserMessageSent(from, to, messageType)
        }
    }
}

func (g *globalMonitoringAdapter) RecordSystemMessageSent(from, to actor.Ref, messageType string) {
    if g.metrics != nil {
        if im, ok := g.metrics.(internalMetrics); ok && im.IsRecording() {
            im.recordSystemMessageSent(from, to, messageType)
        }
    }
}

func (g *globalMonitoringAdapter) RecordUserMessageReceived(actor actor.Ref, messageType string, latency int64) {
    if g.metrics != nil {
        if im, ok := g.metrics.(internalMetrics); ok && im.IsRecording() {
            im.recordUserMessageReceived(actor, messageType, time.Duration(latency))
        }
    }
}

func (g *globalMonitoringAdapter) RecordSystemMessageReceived(actor actor.Ref, messageType string, latency int64) {
    if g.metrics != nil {
        if im, ok := g.metrics.(internalMetrics); ok && im.IsRecording() {
            im.recordSystemMessageReceived(actor, messageType, time.Duration(latency))
        }
    }
}

// 控制方法：安全地代理到内部接口
func (g *globalMonitoringAdapter) StopRecording() {
    if g.metrics != nil {
        if im, ok := g.metrics.(internalMetrics); ok {
            im.StopRecording()
        }
    }
}

func (g *globalMonitoringAdapter) ResumeRecording() {
    if g.metrics != nil {
        if im, ok := g.metrics.(internalMetrics); ok {
            im.ResumeRecording()
        }
    }
}

func (g *globalMonitoringAdapter) IsRecording() bool {
    if g.metrics != nil {
        if im, ok := g.metrics.(internalMetrics); ok {
            return im.IsRecording()
        }
    }
    return false
}

// actorFacade 是 Actor 的门面代理，用于在 Actor 的生命周期中调用 Actor 的方法，
// 它包含两个 Actor 实例：一个是内部的 Actor 实例，一个是对外暴露的 Actor 接口实例，
// 这种设计模式使得内部实现和外部接口可以分离，提高了系统的灵活性和可维护性。
type actorFacade struct {
    actor.Actor                         // 内部 Actor 实例，用于与系统核心交互
    actor            Actor              // 对外暴露的 Actor 接口实例，用于处理用户定义的消息逻辑
    persistentActor  PersistentActor    // 持久化 Actor 实例
    persistenceState *persistence.State // 持久化状态
}
