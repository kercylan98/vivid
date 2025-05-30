package vivid

import (
	"sync"

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

	// 预先检查是否为持久化 Actor，并准备共享的持久化状态
	actorInstance := provider.Provide()
	var sharedPersistenceState *persistence.State
	var smartPersistenceManager *SmartPersistenceManager

	// 检查智能持久化Actor
	if smartActor, ok := actorInstance.(SmartPersistentActor); ok && config.repository != nil && config.enableSmartMode {
		persistenceId := smartActor.GetPersistenceId()
		if persistenceId != "" {
			sharedPersistenceState = persistence.NewState(persistenceId, config.repository)
			smartPersistenceManager = NewSmartPersistenceManager(sharedPersistenceState, config.snapshotPolicy)
		}
	} else if persistentActor, ok := actorInstance.(PersistentActor); ok && config.repository != nil {
		// 传统持久化Actor
		persistenceId := persistentActor.GetPersistenceId()
		if persistenceId != "" {
			sharedPersistenceState = persistence.NewState(persistenceId, config.repository)
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

		// 如果支持智能持久化
		if smartActor, ok := actorInstance.(SmartPersistentActor); ok && smartPersistenceManager != nil {
			facade.smartPersistentActor = smartActor
			facade.smartPersistenceManager = smartPersistenceManager
		} else if persistentActor, ok := actorInstance.(PersistentActor); ok && sharedPersistenceState != nil {
			// 传统持久化
			facade.persistenceState = sharedPersistenceState
			facade.persistentActor = persistentActor
		}

		// 设置 Actor 门面代理的 Actor 方法
		facade.Actor = actor.FN(func(ctx actor.Context) {
			// 内部消息类型转换，处理系统消息和用户消息
			switch msg := ctx.MessageContext().Message().(type) {
			case *actor.OnLaunch:
				waiter.Wait()

				// 智能持久化恢复
				if facade.smartPersistentActor != nil && facade.smartPersistenceManager != nil {
					// 加载持久化数据
					if err := facade.smartPersistenceManager.state.Load(); err == nil {
						smartCtx := newSmartPersistenceContext(facade.smartPersistenceManager)

						// 只有在有可恢复数据时才调用OnRecover
						if smartCtx.CanRecover() {
							facade.smartPersistentActor.OnRecover(smartCtx)
						}
					}
				} else if facade.persistentActor != nil && facade.persistenceState != nil {
					// 传统持久化恢复
					if err := facade.persistenceState.Load(); err == nil {
						persistenceCtx := newPersistenceContext(facade.persistenceState)
						// 只有在有可恢复数据时才调用OnRecover
						if persistenceCtx.CanRecover() {
							facade.persistentActor.OnRecover(persistenceCtx)
						}
					}
				}

				facade.actor.OnReceive(facadeCtx)
			case *actor.OnKill:
				// 在 Actor 被杀死前，保存持久化状态
				if facade.smartPersistenceManager != nil {
					// 智能持久化：强制创建最终快照
					if facade.smartPersistentActor != nil {
						facade.smartPersistenceManager.ForceSnapshot(facade.smartPersistentActor.GetCurrentState())
					}
				} else if facade.persistenceState != nil {
					// 传统持久化
					facade.persistenceState.Persist()
				}
				ctx.MessageContext().HandleWith(&OnKill{m: msg})
			case *actor.OnKilled:
				// 用户无需处理，屏蔽该消息
			case *actor.OnDead:
				ctx.MessageContext().HandleWith(&OnDead{m: msg})
			default:
				facade.actor.OnReceive(facadeCtx)

				// 注意：不在这里自动保存持久化状态
				// 持久化应该由用户在消息处理逻辑中主动调用
			}
		})
		return facade
	})

	// 创建上下文完毕后写入 waiter，通知 Actor 初始化完成（避免竞态问题）
	internalCtx := parent.GenerateContext().GenerateActorContext(system, parent, facadeProvider, *config.config)

	// 根据持久化类型创建上下文
	if smartPersistenceManager != nil {
		smartCtx := newSmartPersistenceContext(smartPersistenceManager)
		facadeCtx = newActorContextWithSmartPersistence(internalCtx, smartCtx)
	} else if sharedPersistenceState != nil {
		persistenceCtx := newPersistenceContext(sharedPersistenceState)
		facadeCtx = newActorContextWithPersistence(internalCtx, persistenceCtx)
	} else {
		facadeCtx = newActorContext(internalCtx)
	}

	waiter.Done()
	return facadeCtx.Ref()
}

// actorFacade 是 Actor 的门面代理，用于在 Actor 的生命周期中调用 Actor 的方法，
// 它包含两个 Actor 实例：一个是内部的 Actor 实例，一个是对外暴露的 Actor 接口实例，
// 这种设计模式使得内部实现和外部接口可以分离，提高了系统的灵活性和可维护性。
type actorFacade struct {
	actor.Actor                                      // 内部 Actor 实例，用于与系统核心交互
	actor                   Actor                    // 对外暴露的 Actor 接口实例，用于处理用户定义的消息逻辑
	persistentActor         PersistentActor          // 持久化 Actor 实例（如果支持传统持久化）
	persistenceState        *persistence.State       // 持久化状态（如果配置了传统持久化）
	smartPersistentActor    SmartPersistentActor     // 智能持久化 Actor 实例（如果支持智能持久化）
	smartPersistenceManager *SmartPersistenceManager // 智能持久化管理器（如果配置了智能持久化）
}
