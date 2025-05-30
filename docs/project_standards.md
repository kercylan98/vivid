# Vivid Actor 项目规范文档

## 项目概述

Vivid 是一个专业级的 Go Actor 模型实现，致力于为工业生产项目提供高性能、高可靠性的并发解决方案。项目采用分层架构设计，通过门面代理模式提供最佳的用户体验。

## 核心设计原则

### 1. 门面代理模式 (Facade Proxy Pattern)

**设计目的**: 在内部复杂性和用户友好性之间提供平衡

```go
// 内部复杂的 actor.Actor 接口
type actor.Actor interface {
// 复杂的内部方法
}

// 用户友好的 Actor 接口  
type Actor interface {
OnReceive(ctx ActorContext) // 简化的用户接口
}

// actorFacade 门面代理
type actorFacade struct {
actor.Actor       // 内部实现
actor       Actor // 用户接口
}
```

**关键规范**:

- 所有用户接口都应通过门面代理实现
- 内部复杂性不应暴露给用户
- 门面代理负责消息类型转换和生命周期管理

### 2. 配置驱动架构

**设计目的**: 提供灵活的功能配置而不影响核心性能

```go
// 配置模式
func (c *ActorConfig) WithFeature(feature Feature) *ActorConfig {
c.feature = feature
return c // 链式调用
}

// 使用模式
system.ActorOf(provider, func (config *ActorConfig) {
config.WithName("myActor").
WithPersistence(repo).
WithSupervisor(supervisor)
})
```

**关键规范**:

- 所有功能都应通过配置启用
- 配置方法使用 `With` 前缀
- 支持链式调用
- 默认配置应满足大多数使用场景

### 3. 渐进式增强

**设计目的**: 支持从简单到复杂的使用场景

```go
// 基础接口
type Actor interface {
OnReceive(ctx ActorContext)
}

// 增强接口
type PersistentActor interface {
Actor
OnRecover(ctx PersistenceContext)
GetPersistenceId() string
}

// 智能增强接口
type SmartPersistentActor interface {
Actor
OnRecover(ctx SmartPersistenceContext)
GetPersistenceId() string
GetCurrentState() any
ApplyEvent(event Event)
}
```

**关键规范**:

- 接口应支持组合和扩展
- 高级功能不应影响基础功能的性能
- 向后兼容性是强制要求

## 代码组织结构

### 目录结构

```
src/vivid/
├── actor.go                 # 核心Actor接口和门面代理
├── actor_context.go         # Actor上下文接口
├── actor_config.go          # Actor配置接口
├── actor_system.go          # Actor系统接口
├── persistence.go           # 持久化接口
├── persistence_policy.go    # 持久化策略
├── messages.go             # 系统消息定义
├── alias.go                # 类型别名
└── internal/               # 内部实现
    ├── core/               # 核心组件
    │   ├── actor/          # Actor核心实现
    │   ├── system/         # 系统核心实现
    │   └── future/         # Future实现
    └── actx/               # 上下文实现
```

### 接口设计规范

#### 1. 接口命名

- 核心接口使用简洁名称: `Actor`, `ActorContext`, `ActorSystem`
- 增强接口使用描述性前缀: `SmartPersistentActor`, `PersistentActor`
- 内部接口在 `internal` 包中，以 `actor.` 前缀

#### 2. 方法命名

```go
// 配置方法: With + 功能名
func WithPersistence(repo Repository) *ActorConfig

// 操作方法: 动词 + 名词
func Tell(target ActorRef, message Message)
func Ask(target ActorRef, message Message) Future

// 获取方法: Get + 属性名
func GetPersistenceId() string
func GetSnapshot() Snapshot

// 判断方法: 动词 + 条件
func CanRecover() bool
func ShouldCreateSnapshot() bool
```

#### 3. 上下文接口设计

```go
// 基础上下文
type context interface {
// 基础功能
}

// 扩展上下文
type ActorContext interface {
context
actor.TimingContext // 组合其他上下文

// Actor特定功能
}
```

### 错误处理规范

#### 1. 错误类型设计

```go
// 自定义错误类型
type PersistenceError struct {
Op    string // 操作类型
Err   error  // 原始错误
Actor string // Actor标识
}

func (e *PersistenceError) Error() string {
return fmt.Sprintf("persistence %s failed for actor %s: %v", e.Op, e.Actor, e.Err)
}
```

#### 2. 错误处理策略

- 内部错误应被转换为用户友好的错误
- 使用 `fmt.Errorf` 包装错误，保留错误链
- 关键操作失败应提供恢复建议

### 测试规范

#### 1. 测试文件组织

```go
// 基础测试
func TestActorBasic(t *testing.T) {}

// 功能测试  
func TestActorPersistence(t *testing.T) {}

// 集成测试
func TestActorSystemIntegration(t *testing.T) {}

// 性能测试
func BenchmarkActorThroughput(b *testing.B) {}
```

#### 2. 测试数据结构

```go
// 测试用的Actor实现
type TestActor struct {
state *TestState
id    string
}

// 使用Test前缀避免与生产代码冲突
type TestState struct {
Count int
}

type TestEvent struct {
Delta int
}
```

## 持久化模块规范

### 1. 接口层次设计

```go
// 基础层 - 最小功能
type PersistentActor interface {
Actor
OnRecover(ctx PersistenceContext)
GetPersistenceId() string
}

// 智能层 - 自动化功能
type SmartPersistentActor interface {
Actor
OnRecover(ctx SmartPersistenceContext)
GetPersistenceId() string
GetCurrentState() any // 用于自动快照
ApplyEvent(event Event) // 用于事件重放
}
```

### 2. 策略模式应用

```go
// 快照策略接口
type SnapshotStrategy interface {
ShouldCreateSnapshot(eventCount int, lastSnapshot time.Time, currentState any) bool
}

// 序列化策略接口
type Serializer interface {
Serialize(obj any) ([]byte, error)
Deserialize(data []byte, target any) error
DeepCopy(obj any) (any, error)
}
```

### 3. 智能化管理

```go
// 智能持久化管理器
type SmartPersistenceManager struct {
state           *persistence.State
policy          *AutoSnapshotPolicy
serializer      Serializer
eventCount      int
lastSnapshot    time.Time
}

// 自动化操作
func (m *SmartPersistenceManager) PersistEvent(event Event, currentState any) error {
// 1. 持久化事件
// 2. 检查快照策略
// 3. 自动创建快照
// 4. 保存到仓库
}
```

## 性能优化规范

### 1. 内存管理

- 避免不必要的内存分配
- 使用对象池管理频繁创建的对象
- 及时释放不再使用的资源

### 2. 并发安全

- 所有公开接口必须是并发安全的
- 使用适当的同步原语（sync.Mutex, sync.RWMutex, atomic）
- 避免数据竞争

### 3. 性能监控

```go
// 提供性能指标接口
type Metrics interface {
MessageThroughput() int64
ErrorRate() float64
AverageLatency() time.Duration
}
```

## 版本兼容性规范

### 1. API稳定性

- 公开接口一旦发布，必须保持向后兼容
- 新功能通过新接口添加
- 废弃功能使用 `@deprecated` 标记

### 2. 版本管理

- 使用语义化版本控制 (SemVer)
- 主版本号变更表示破坏性变更
- 次版本号变更表示新功能添加
- 修订版本号表示错误修复

## 文档规范

### 1. 代码注释

```go
// Package vivid 提供高性能的Actor模型实现
//
// Vivid采用门面代理模式，为用户提供简洁的API接口，
// 同时在内部实现复杂的并发控制和生命周期管理。
//
// 基本使用示例:
//   system := vivid.NewActorSystem()
//   actorRef := system.ActorOf(func() vivid.Actor {
//       return &MyActor{}
//   })
//   system.Tell(actorRef, "hello")
package vivid

// Actor 是系统中的基本计算单元接口
//
// Actor封装了状态和行为，通过消息传递与其他Actor通信。
// 所有Actor必须实现OnReceive方法来处理接收到的消息。
//
// 实现示例:
//   type MyActor struct{}
//   func (a *MyActor) OnReceive(ctx vivid.ActorContext) {
//       switch msg := ctx.Message().(type) {
//       case string:
//           fmt.Printf("Received: %s\n", msg)
//       }
//   }
type Actor interface {
    OnReceive(ctx ActorContext)
}
```

### 2. 示例代码

- 每个重要功能都应提供完整的示例
- 示例应该可以直接运行
- 包含错误处理和最佳实践

### 3. 架构文档

- 详细说明设计决策和权衡
- 提供性能基准和使用指南
- 包含故障排除指南

## 最佳实践

### 1. Actor设计

- Actor应该保持单一职责
- 状态应该是私有的，只通过消息修改
- 避免在Actor之间共享可变状态

### 2. 消息设计

- 消息应该是不可变的
- 使用结构体而不是原始类型
- 为消息提供明确的语义

### 3. 错误处理

- 使用监管者策略处理Actor异常
- 提供graceful degradation机制
- 记录足够的上下文信息用于调试

### 4. 持久化

- 选择合适的快照策略
- 确保事件的向后兼容性
- 实现proper的序列化/反序列化

这个规范文档将随着项目的发展不断更新和完善。 