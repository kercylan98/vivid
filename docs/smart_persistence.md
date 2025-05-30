# 智能持久化功能

## 概述

Vivid Actor 系统现在提供了两种持久化模式：

1. **传统持久化** (`PersistentActor`) - 手动管理快照和事件
2. **智能持久化** (`SmartPersistentActor`) - 自动化快照管理和深拷贝

智能持久化是对传统持久化的增强，提供了更好的用户体验和自动化功能。

## 核心特性

### 🚀 自动快照管理

- 基于事件数量阈值自动创建快照
- 基于时间间隔自动创建快照
- 可配置的快照策略
- Actor关闭时自动创建最终快照

### 🔄 自动深拷贝

- 自动深拷贝状态避免引用问题
- 支持JSON序列化的自动深拷贝
- 可扩展的序列化器接口

### 🎯 智能恢复

- 自动从快照恢复
- 自动重放快照后的事件
- 用户友好的恢复接口

### ⚙️ 策略可配置

- 灵活的快照策略配置
- 支持自定义序列化器
- 向后兼容传统持久化

## 接口设计

### SmartPersistentActor 接口

```go
type SmartPersistentActor interface {
Actor

// OnRecover 当 Actor 从持久化存储中恢复时被调用
OnRecover(ctx SmartPersistenceContext)

// GetPersistenceId 返回此 Actor 的持久化标识符
GetPersistenceId() string

// GetCurrentState 获取当前状态，用于智能快照
GetCurrentState() any

// ApplyEvent 应用事件到当前状态
ApplyEvent(event persistence.Event)
}
```

### SmartPersistenceContext 接口

```go
type SmartPersistenceContext interface {
PersistenceContext

// PersistWithState 持久化事件并自动管理快照
PersistWithState(event persistence.Event, currentState any) error

// ForceSnapshot 强制创建快照
ForceSnapshot(state any) error

// GetSnapshotPolicy 获取当前的快照策略
GetSnapshotPolicy() *AutoSnapshotPolicy

// SetSnapshotPolicy 设置快照策略
SetSnapshotPolicy(policy *AutoSnapshotPolicy)

// GetEventCount 获取自上次快照以来的事件数量
GetEventCount() int

// GetLastSnapshotTime 获取上次快照时间
GetLastSnapshotTime() time.Time
}
```

## 配置选项

### 基本配置

```go
// 使用默认智能持久化
config.WithDefaultSmartPersistence(repository)

// 自定义智能持久化
config.WithSmartPersistence(repository, policy, serializer)
```

### 快照策略配置

```go
policy := &vivid.AutoSnapshotPolicy{
EventThreshold:          10, // 每10个事件创建快照
TimeThreshold:           5 * time.Minute, // 每5分钟创建快照
StateChangeThreshold:    0.3,             // 状态变化30%时创建快照
ForceSnapshotOnShutdown: true, // 关闭时强制快照
}
```

## 使用示例

### 智能持久化Actor实现

```go
type BankAccountActor struct {
state *BankAccountState
}

func (a *BankAccountActor) GetPersistenceId() string {
return fmt.Sprintf("bank-account-%s", a.state.AccountID)
}

func (a *BankAccountActor) GetCurrentState() any {
return a.state
}

func (a *BankAccountActor) ApplyEvent(event any) {
switch e := event.(type) {
case *DepositEvent:
a.state.Balance += e.Amount
case *WithdrawEvent:
a.state.Balance -= e.Amount
}
}

func (a *BankAccountActor) OnRecover(ctx vivid.SmartPersistenceContext) {
// 从快照恢复
if snapshot := ctx.GetSnapshot(); snapshot != nil {
if state, ok := snapshot.(*BankAccountState); ok {
a.state = state
}
}

// 重放事件
events := ctx.GetEvents()
for _, event := range events {
a.ApplyEvent(event)
}
}

func (a *BankAccountActor) OnReceive(ctx vivid.ActorContext) {
switch msg := ctx.Message().(type) {
case *DepositEvent:
// 智能持久化 - 自动管理快照
if smartCtx := ctx.Persistence().(vivid.SmartPersistenceContext); smartCtx != nil {
err := smartCtx.PersistWithState(msg, a.GetCurrentState())
if err != nil {
return
}
}

// 更新状态
a.state.Balance += msg.Amount
ctx.Reply(a.state.Balance)
}
}
```

### 创建智能持久化Actor

```go
// 创建Actor系统
system := vivid.NewActorSystem()
system.StartP()

// 创建持久化仓库
repository := persistencerepos.NewMemory()

// 创建自定义快照策略
policy := &vivid.AutoSnapshotPolicy{
EventThreshold:          5,
TimeThreshold:           30 * time.Second,
ForceSnapshotOnShutdown: true,
}

// 创建智能持久化Actor
actorRef := system.ActorOf(func () vivid.Actor {
return NewBankAccountActor("ACC001")
}, func (config *vivid.ActorConfig) {
config.WithName("bank-account").
WithSmartPersistence(repository, policy, nil)
})
```

## 与传统持久化的对比

| 特性    | 传统持久化 | 智能持久化 |
|-------|-------|-------|
| 快照管理  | 手动    | 自动    |
| 深拷贝   | 手动    | 自动    |
| 策略配置  | 无     | 支持    |
| 用户复杂度 | 高     | 低     |
| 性能开销  | 低     | 中等    |
| 向后兼容  | -     | 完全兼容  |

## 最佳实践

### 1. 状态设计

- 保持状态结构简单，便于序列化
- 使用JSON标签确保序列化兼容性
- 避免在状态中包含不可序列化的字段

### 2. 事件设计

- 事件应该是不可变的
- 包含足够的信息用于状态重建
- 保持事件的向后兼容性

### 3. 快照策略

- 根据业务需求调整事件阈值
- 考虑状态大小和恢复时间的平衡
- 在高频操作场景下适当降低阈值

### 4. 错误处理

- 在持久化失败时提供适当的错误处理
- 考虑实现重试机制
- 记录持久化相关的错误日志

## 性能考虑

### 内存使用

- 智能持久化会创建状态的深拷贝
- 大状态对象可能增加内存使用
- 考虑使用自定义序列化器优化

### CPU开销

- JSON序列化/反序列化有一定开销
- 可以通过自定义序列化器优化
- 快照频率影响CPU使用

### 存储开销

- 自动快照可能增加存储使用
- 通过合理的快照策略控制
- 考虑实现快照清理机制

## 故障排除

### 常见问题

1. **状态恢复不正确**
    - 检查GetCurrentState()返回的状态
    - 确保ApplyEvent()正确实现
    - 验证序列化/反序列化逻辑

2. **快照未自动创建**
    - 检查快照策略配置
    - 确认使用PersistWithState()方法
    - 验证事件计数是否达到阈值

3. **性能问题**
    - 调整快照策略参数
    - 考虑使用自定义序列化器
    - 监控内存和CPU使用

### 调试技巧

- 使用GetEventCount()监控事件数量
- 通过GetLastSnapshotTime()检查快照时间
- 在OnRecover()中添加日志输出
- 使用ForceSnapshot()手动触发快照

这个智能持久化功能为Vivid Actor系统提供了工业级的持久化解决方案，在保持高性能的同时大大简化了用户的使用复杂度。 