# Actor 持久化功能

Actor 持久化功能允许 Actor 保存和恢复其状态，支持事件溯源模式。Actor 的状态通过快照（Snapshot）保存，行为变化通过事件（Event）记录。

## 核心概念

### PersistentActor 接口

实现 `PersistentActor` 接口的 Actor 可以使用持久化功能：

```go
type PersistentActor interface {
Actor

// OnRecover 当 Actor 从持久化存储中恢复时被调用
OnRecover(ctx PersistenceContext)

// GetPersistenceId 返回此 Actor 的持久化标识符
GetPersistenceId() string
}
```

### PersistenceContext 接口

持久化上下文提供了与持久化存储交互的方法：

```go
type PersistenceContext interface {
// GetSnapshot 获取当前的快照数据
GetSnapshot() persistence.Snapshot

// GetEvents 获取自最后一次快照以来的所有事件
GetEvents() []persistence.Event

// Persist 持久化一个事件
Persist(event persistence.Event)

// SaveSnapshot 保存当前状态的快照
SaveSnapshot(snapshot persistence.Snapshot)

// CanRecover 检查是否有可恢复的数据
CanRecover() bool
}
```

## 使用示例

### 1. 创建持久化 Actor

```go
// 定义状态结构
type CounterState struct {
Count int `json:"count"`
}

// 定义事件结构
type IncrementEvent struct {
Delta int `json:"delta"`
}

// 实现持久化 Actor
type CounterActor struct {
state *CounterState
}

func (c *CounterActor) GetPersistenceId() string {
return "counter-actor-1"
}

func (c *CounterActor) OnRecover(ctx vivid.PersistenceContext) {
// 恢复快照
if snapshot := ctx.GetSnapshot(); snapshot != nil {
if state, ok := snapshot.(*CounterState); ok {
c.state = state
}
} else {
// 初始化默认状态
c.state = &CounterState{Count: 0}
}

// 重放事件
events := ctx.GetEvents()
for _, event := range events {
if incrementEvent, ok := event.(*IncrementEvent); ok {
c.state.Count += incrementEvent.Delta
}
}
}

func (c *CounterActor) OnReceive(ctx vivid.ActorContext) {
switch msg := ctx.Message().(type) {
case *IncrementEvent:
// 持久化事件
if persistenceCtx := ctx.Persistence(); persistenceCtx != nil {
persistenceCtx.Persist(msg)
}

// 更新状态
c.state.Count += msg.Delta

// 每 10 次增量就创建一个快照
if c.state.Count%10 == 0 {
if persistenceCtx := ctx.Persistence(); persistenceCtx != nil {
persistenceCtx.SaveSnapshot(c.state)
}
}

case string:
if msg == "get" {
ctx.Reply(c.state.Count)
}
}
}
```

### 2. 配置持久化仓库

```go
// 创建内存持久化仓库
repository := persistencerepos.NewMemory()

// 创建 Actor 系统
system := vivid.NewActorSystem()
system.StartP()

// 创建持久化 Actor
counterRef := system.ActorOf(func () vivid.Actor {
return &CounterActor{}
}, func (config *vivid.ActorConfig) {
config.WithName("counter").WithPersistence(repository)
})
```

### 3. 发送消息

```go
// 发送事件
system.Tell(counterRef, &IncrementEvent{Delta: 5})

// 查询状态
future := system.Ask(counterRef, "get", time.Second)
result, _ := future.Result()
fmt.Printf("当前计数: %v\n", result)
```

## 持久化仓库

### 内存仓库

适用于测试和开发环境：

```go
repository := persistencerepos.NewMemory()
```

### 自定义仓库

实现 `persistence.Repository` 接口来创建自定义持久化仓库：

```go
type Repository interface {
Save(persistenceId string, snapshot Snapshot, events []Event) error
Load(persistenceId string) (Snapshot, []Event, error)
}
```

## 最佳实践

1. **持久化 ID 设计**：使用具有业务意义的唯一标识符作为持久化 ID
2. **快照策略**：定期创建快照以减少事件重放时间
3. **事件设计**：保持事件的不可变性和向后兼容性
4. **错误处理**：在 OnRecover 方法中处理数据格式变化和错误情况
5. **性能考虑**：避免在每个消息处理后都保存快照，根据业务需求制定合适的快照策略

## 注意事项

- 只有实现了 `PersistentActor` 接口且配置了持久化仓库的 Actor 才能使用持久化功能
- 持久化操作是异步的，系统会在适当的时机自动保存状态
- 在 Actor 被杀死时，系统会自动保存最终状态
- 事件和快照的序列化/反序列化需要确保类型兼容性 