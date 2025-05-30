package vivid

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/src/persistence/persistencerepos"
)

// TestCounterState 测试计数器状态
type TestCounterState struct {
	Count int
}

// TestIncrementEvent 测试增量事件
type TestIncrementEvent struct {
	Delta int
}

// TestPersistentActorImpl 测试持久化 Actor 实现
type TestPersistentActorImpl struct {
	state *TestCounterState
	id    string
}

func NewTestPersistentActorImpl(id string) *TestPersistentActorImpl {
	return &TestPersistentActorImpl{
		state: &TestCounterState{Count: 0},
		id:    id,
	}
}

func (t *TestPersistentActorImpl) GetPersistenceId() string {
	return t.id
}

func (t *TestPersistentActorImpl) OnRecover(ctx PersistenceContext) {
	// 恢复快照
	if snapshot := ctx.GetSnapshot(); snapshot != nil {
		if state, ok := snapshot.(*TestCounterState); ok {
			t.state = state
			// 添加调试信息
			println("DEBUG: Recovered from snapshot with count:", t.state.Count)
		}
	}

	// 重放事件
	events := ctx.GetEvents()
	println("DEBUG: Replaying", len(events), "events")
	for i, event := range events {
		if incrementEvent, ok := event.(*TestIncrementEvent); ok {
			t.state.Count += incrementEvent.Delta
			println("DEBUG: Replayed event", i+1, "delta:", incrementEvent.Delta, "new count:", t.state.Count)
		}
	}
	println("DEBUG: Final count after recovery:", t.state.Count)
}

func (t *TestPersistentActorImpl) OnReceive(ctx ActorContext) {
	switch msg := ctx.Message().(type) {
	case *TestIncrementEvent:
		// 持久化事件
		if persistenceCtx := ctx.Persistence(); persistenceCtx != nil {
			persistenceCtx.Persist(msg)
			println("DEBUG: Persisted event with delta:", msg.Delta)
		}

		// 更新状态
		t.state.Count += msg.Delta
		println("DEBUG: Actor state updated, new count:", t.state.Count)

		// 回复当前计数
		ctx.Reply(t.state.Count)

	case string:
		if msg == "snapshot" {
			if persistenceCtx := ctx.Persistence(); persistenceCtx != nil {
				// 创建状态的深拷贝而不是传递指针
				snapshotCopy := &TestCounterState{Count: t.state.Count}
				persistenceCtx.SaveSnapshot(snapshotCopy)
				println("DEBUG: Saved snapshot with count:", snapshotCopy.Count)
			}
			ctx.Reply("snapshotted")
		} else if msg == "get" {
			println("DEBUG: Get request, returning count:", t.state.Count)
			ctx.Reply(t.state.Count)
		}
	}
}

func TestPersistentActorBasic(t *testing.T) {
	// 创建内存持久化仓库
	repository := persistencerepos.NewMemory()

	// 创建 Actor 系统
	system := NewActorSystem()
	system.StartP()
	defer system.StopP()

	// 创建持久化 Actor
	actorRef := system.ActorOf(func() Actor {
		return NewTestPersistentActorImpl("test-actor-1")
	}, func(config *ActorConfig) {
		config.WithName("test-persistent").WithPersistence(repository)
	})

	// 发送增量事件
	future1 := system.Ask(actorRef, &TestIncrementEvent{Delta: 5}, time.Second)
	result1, _ := future1.Result()
	if result1 != 5 {
		t.Errorf("Expected count to be 5, got %v", result1)
	}

	future2 := system.Ask(actorRef, &TestIncrementEvent{Delta: 3}, time.Second)
	result2, _ := future2.Result()
	if result2 != 8 {
		t.Errorf("Expected count to be 8, got %v", result2)
	}

	// 创建快照
	future3 := system.Ask(actorRef, "snapshot", time.Second)
	result3, _ := future3.Result()
	if result3 != "snapshotted" {
		t.Errorf("Expected snapshot confirmation, got %v", result3)
	}

	// 继续发送事件
	future4 := system.Ask(actorRef, &TestIncrementEvent{Delta: 2}, time.Second)
	result4, _ := future4.Result()
	if result4 != 10 {
		t.Errorf("Expected count to be 10, got %v", result4)
	}
}

func TestPersistentActorRecovery(t *testing.T) {
	// 创建内存持久化仓库
	repository := persistencerepos.NewMemory()

	// 创建 Actor 系统
	system := NewActorSystem()
	system.StartP()
	defer system.StopP()

	// 第一个 Actor - 保存一些数据
	actorRef1 := system.ActorOf(func() Actor {
		return NewTestPersistentActorImpl("recovery-test-actor")
	}, func(config *ActorConfig) {
		config.WithName("recovery-test-1").WithPersistence(repository)
	})

	// 发送一些事件并等待完成
	future1 := system.Ask(actorRef1, &TestIncrementEvent{Delta: 10}, time.Second)
	future1.Result() // 等待完成

	future2 := system.Ask(actorRef1, &TestIncrementEvent{Delta: 5}, time.Second)
	future2.Result() // 等待完成，此时 count = 15

	// 创建快照并等待完成
	future3 := system.Ask(actorRef1, "snapshot", time.Second)
	future3.Result() // 等待完成，快照保存 count = 15

	// 发送更多事件（这些会在快照之后）并等待完成
	future4 := system.Ask(actorRef1, &TestIncrementEvent{Delta: 3}, time.Second)
	future4.Result() // 等待完成，count = 18

	future5 := system.Ask(actorRef1, &TestIncrementEvent{Delta: 2}, time.Second)
	future5.Result() // 等待完成，count = 20

	// 等待一段时间确保所有消息都被处理
	time.Sleep(100 * time.Millisecond)

	// 创建第二个 Actor 使用相同的 persistence ID - 应该恢复状态
	actorRef2 := system.ActorOf(func() Actor {
		return NewTestPersistentActorImpl("recovery-test-actor")
	}, func(config *ActorConfig) {
		config.WithName("recovery-test-2").WithPersistence(repository)
	})

	// 等待一段时间让恢复过程完成
	time.Sleep(100 * time.Millisecond)

	// 检查恢复的状态
	future := system.Ask(actorRef2, "get", time.Second)
	result, _ := future.Result()

	// 应该恢复到 15 (快照) + 3 + 2 (事件重放) = 20
	if result != 20 {
		t.Errorf("Expected recovered count to be 20, got %v", result)
	}
}
