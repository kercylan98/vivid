package vivid

import (
    "testing"
    "time"

    "github.com/kercylan98/vivid/src/persistence/persistencerepos"
)

// TestSmartCounterState 测试智能计数器状态
type TestSmartCounterState struct {
    Count int
}

// TestSmartIncrementEvent 测试智能增量事件
type TestSmartIncrementEvent struct {
    Delta int
}

// TestSmartPersistentActorImpl 测试智能持久化 Actor 实现
type TestSmartPersistentActorImpl struct {
    state *TestSmartCounterState
    id    string
}

func NewTestSmartPersistentActorImpl(id string) *TestSmartPersistentActorImpl {
    return &TestSmartPersistentActorImpl{
        state: &TestSmartCounterState{Count: 0},
        id:    id,
    }
}

func (t *TestSmartPersistentActorImpl) GetPersistenceId() string {
    return t.id
}

func (t *TestSmartPersistentActorImpl) GetCurrentState() any {
    return t.state
}

func (t *TestSmartPersistentActorImpl) ApplyEvent(event any) {
    if incrementEvent, ok := event.(*TestSmartIncrementEvent); ok {
        t.state.Count += incrementEvent.Delta
    }
}

func (t *TestSmartPersistentActorImpl) OnRecover(ctx SmartPersistenceContext) {
    // 从快照恢复
    if snapshot := ctx.GetSnapshot(); snapshot != nil {
        if state, ok := snapshot.(*TestSmartCounterState); ok {
            t.state = state
        }
    }

    // 重放事件
    events := ctx.GetEvents()
    for _, event := range events {
        t.ApplyEvent(event)
    }
}

func (t *TestSmartPersistentActorImpl) OnReceive(ctx ActorContext) {
    switch msg := ctx.Message().(type) {
    case *TestSmartIncrementEvent:
        // 使用智能持久化
        if smartCtx := ctx.Persistence().(SmartPersistenceContext); smartCtx != nil {
            err := smartCtx.PersistWithState(msg, t.GetCurrentState())
            if err != nil {
                return
            }
        }

        // 更新状态
        t.state.Count += msg.Delta
        ctx.Reply(t.state.Count)

    case string:
        if msg == "get" {
            ctx.Reply(t.state.Count)
        }
    }
}

func TestSmartPersistentActorBasic(t *testing.T) {
    // 创建内存持久化仓库
    repository := persistencerepos.NewMemory()

    // 创建 Actor 系统
    system := NewActorSystem()
    system.StartP()
    defer system.StopP()

    // 创建智能持久化 Actor
    actorRef := system.ActorOf(func() Actor {
        return NewTestSmartPersistentActorImpl("smart-test-actor-1")
    }, func(config *ActorConfig) {
        config.WithName("test-smart-persistent").WithDefaultSmartPersistence(repository)
    })

    // 发送增量事件
    future1 := system.Ask(actorRef, &TestSmartIncrementEvent{Delta: 5}, time.Second)
    result1, _ := future1.Result()
    if result1 != 5 {
        t.Errorf("Expected count to be 5, got %v", result1)
    }

    future2 := system.Ask(actorRef, &TestSmartIncrementEvent{Delta: 3}, time.Second)
    result2, _ := future2.Result()
    if result2 != 8 {
        t.Errorf("Expected count to be 8, got %v", result2)
    }

    // 等待一段时间确保持久化完成
    time.Sleep(100 * time.Millisecond)

    // 创建第二个 Actor 使用相同的 persistence ID - 应该恢复状态
    actorRef2 := system.ActorOf(func() Actor {
        return NewTestSmartPersistentActorImpl("smart-test-actor-1")
    }, func(config *ActorConfig) {
        config.WithName("test-smart-persistent-2").WithDefaultSmartPersistence(repository)
    })

    // 等待恢复完成
    time.Sleep(100 * time.Millisecond)

    // 检查恢复的状态
    future := system.Ask(actorRef2, "get", time.Second)
    result, _ := future.Result()
    if result != 8 {
        t.Errorf("Expected recovered count to be 8, got %v", result)
    }
}
