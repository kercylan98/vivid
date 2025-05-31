package vivid_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid/src/persistence/persistencerepos"
	"github.com/kercylan98/vivid/src/vivid"
)

// =============================================================================
// 测试数据结构
// =============================================================================

// TestState 表示测试Actor的状态
type TestState struct {
	Counter      int64    `json:"counter"`
	History      []string `json:"history"`
	LastUpdate   string   `json:"last_update"`
	RestartCount int      `json:"restart_count"`
}

// TestEvent 表示测试事件
type TestEvent struct {
	Type      string `json:"type"`
	Value     int64  `json:"value"`
	Timestamp string `json:"timestamp"`
}

// GetStateCommand 获取状态命令
type GetStateCommand struct{}

// PanicCommand panic命令
type PanicCommand struct{}

// =============================================================================
// 测试Actor实现
// =============================================================================

// PersistentTestActor 简洁的持久化测试Actor
type PersistentTestActor struct {
	id    string
	state *TestState
}

// NewPersistentTestActor 创建新的测试Actor
func NewPersistentTestActor(id string) *PersistentTestActor {
	return &PersistentTestActor{
		id: id,
		state: &TestState{
			Counter:      0,
			History:      make([]string, 0),
			LastUpdate:   time.Now().Format("15:04:05.000"),
			RestartCount: 0,
		},
	}
}

// GetPersistenceId 实现PersistentActor接口
func (a *PersistentTestActor) GetPersistenceId() string {
	return a.id
}

// GetCurrentState 实现PersistentActor接口
func (a *PersistentTestActor) GetCurrentState() any {
	return a.state
}

// RestoreState 实现PersistentActor接口
func (a *PersistentTestActor) RestoreState(state any) {
	if restoredState, ok := state.(*TestState); ok {
		a.state = restoredState
	}
}

// OnReceive 实现Actor接口
func (a *PersistentTestActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		// 检查是否为重启
		if msg.Restarted() {
			a.state.RestartCount++
			a.state.LastUpdate = time.Now().Format("15:04:05.000")
		}

	case *TestEvent:
		a.handleTestEvent(ctx, msg)

	case string:
		a.handleStringCommand(ctx, msg)

	case int64:
		a.handleCounterIncrement(ctx, msg)

	case *GetStateCommand:
		ctx.Reply(a.state)

	case *PanicCommand:
		panic(errors.New("测试panic"))
	}
}

// handleTestEvent 处理测试事件
func (a *PersistentTestActor) handleTestEvent(ctx vivid.ActorContext, event *TestEvent) {
	if pCtx := ctx.Persistence(); pCtx != nil {
		if err := pCtx.Persist(event); err == nil {
			a.state.Counter += event.Value
			a.state.History = append(a.state.History, fmt.Sprintf("%s:%d", event.Type, event.Value))
			a.state.LastUpdate = event.Timestamp
		}
	}
}

// handleStringCommand 处理字符串命令
func (a *PersistentTestActor) handleStringCommand(ctx vivid.ActorContext, cmd string) {
	if pCtx := ctx.Persistence(); pCtx != nil {
		if err := pCtx.Persist(cmd); err == nil {
			a.state.History = append(a.state.History, cmd)
			a.state.LastUpdate = time.Now().Format("15:04:05.000")
		}
	}
}

// handleCounterIncrement 处理计数器增量
func (a *PersistentTestActor) handleCounterIncrement(ctx vivid.ActorContext, increment int64) {
	if pCtx := ctx.Persistence(); pCtx != nil {
		if err := pCtx.Persist(increment); err == nil {
			a.state.Counter += increment
			a.state.LastUpdate = time.Now().Format("15:04:05.000")
		}
	}
}

// =============================================================================
// 测试助手函数
// =============================================================================

// createTestSystem 创建测试系统
func createTestSystem(_ *testing.T) (vivid.ActorSystem, func()) {
	system := vivid.NewActorSystem().StartP()
	return system, func() {
		system.PoisonStopP()
	}
}

// waitForCondition 等待条件满足
func waitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", message)
}

// assertStateEquals 断言状态相等
func assertStateEquals(t *testing.T, expected, actual *TestState, context string) {
	t.Helper()

	if expected.Counter != actual.Counter {
		t.Errorf("%s: Counter不匹配, 期望: %d, 实际: %d", context, expected.Counter, actual.Counter)
	}

	if len(expected.History) != len(actual.History) {
		t.Errorf("%s: History长度不匹配, 期望: %d, 实际: %d", context, len(expected.History), len(actual.History))
		return
	}

	for i, expectedItem := range expected.History {
		if i < len(actual.History) && expectedItem != actual.History[i] {
			t.Errorf("%s: History[%d]不匹配, 期望: %s, 实际: %s", context, i, expectedItem, actual.History[i])
		}
	}
}

// =============================================================================
// 基础持久化功能测试
// =============================================================================

func TestBasicPersistence(t *testing.T) {
	system, cleanup := createTestSystem(t)
	defer cleanup()

	repo := persistencerepos.NewMemory()

	t.Run("基本持久化和恢复", func(t *testing.T) {
		// 创建第一个Actor
		actor1 := system.ActorOf(func() vivid.Actor {
			return NewPersistentTestActor("test-basic")
		}, func(config *vivid.ActorConfig) {
			config.WithName("basic-1").WithDefaultPersistence(repo)
		})

		// 发送一系列事件
		system.Tell(actor1, int64(10))
		system.Tell(actor1, int64(20))
		system.Tell(actor1, "cmd1")
		system.Tell(actor1, int64(-5))

		// 等待处理完成
		time.Sleep(100 * time.Millisecond)

		// 验证第一个Actor的状态
		future1 := system.Ask(actor1, &GetStateCommand{}, 1*time.Second)
		result1, err := future1.Result()
		if err != nil {
			t.Fatalf("获取第一个Actor状态失败: %v", err)
		}

		state1 := result1.(*TestState)
		expectedState1 := &TestState{
			Counter:      25, // 10 + 20 - 5
			History:      []string{"cmd1"},
			RestartCount: 0,
		}
		assertStateEquals(t, expectedState1, state1, "第一个Actor")

		// 关闭第一个Actor
		system.PoisonKill(actor1, "测试关闭")
		time.Sleep(100 * time.Millisecond)

		// 创建第二个相同ID的Actor
		actor2 := system.ActorOf(func() vivid.Actor {
			return NewPersistentTestActor("test-basic")
		}, func(config *vivid.ActorConfig) {
			config.WithName("basic-2").WithDefaultPersistence(repo)
		})

		// 等待恢复完成
		time.Sleep(100 * time.Millisecond)

		// 验证恢复的状态
		future2 := system.Ask(actor2, &GetStateCommand{}, 1*time.Second)
		result2, err := future2.Result()
		if err != nil {
			t.Fatalf("获取第二个Actor状态失败: %v", err)
		}

		state2 := result2.(*TestState)
		assertStateEquals(t, expectedState1, state2, "恢复后的Actor")

		// 继续发送事件验证功能正常
		system.Tell(actor2, int64(5))
		system.Tell(actor2, "cmd2")
		time.Sleep(100 * time.Millisecond)

		future3 := system.Ask(actor2, &GetStateCommand{}, 1*time.Second)
		result3, err := future3.Result()
		if err != nil {
			t.Fatalf("获取最终状态失败: %v", err)
		}

		finalState := result3.(*TestState)
		expectedFinalState := &TestState{
			Counter:      30, // 25 + 5
			History:      []string{"cmd1", "cmd2"},
			RestartCount: 0,
		}
		assertStateEquals(t, expectedFinalState, finalState, "最终状态")
	})
}

// =============================================================================
// 快照策略测试
// =============================================================================

func TestSnapshotPolicy(t *testing.T) {
	system, cleanup := createTestSystem(t)
	defer cleanup()

	repo := persistencerepos.NewMemory()

	t.Run("自定义快照策略", func(t *testing.T) {
		// 创建自定义快照策略 - 每2个事件创建快照
		policy := &vivid.AutoSnapshotPolicy{
			EventThreshold:          2,
			TimeThreshold:           1 * time.Hour, // 时间阈值设置很大，确保只通过事件数量触发
			StateChangeThreshold:    1.0,           // 不通过状态变化触发
			ForceSnapshotOnShutdown: true,
		}

		actor := system.ActorOf(func() vivid.Actor {
			return NewPersistentTestActor("test-snapshot")
		}, func(config *vivid.ActorConfig) {
			config.WithName("snapshot-test").WithPersistence(repo, policy)
		})

		// 发送事件，应该在第2个事件后创建快照
		system.Tell(actor, int64(1)) // 事件1
		system.Tell(actor, int64(2)) // 事件2 - 应该触发快照
		system.Tell(actor, int64(3)) // 事件3
		system.Tell(actor, int64(4)) // 事件4 - 应该再次触发快照

		time.Sleep(200 * time.Millisecond)

		// 验证状态
		future := system.Ask(actor, &GetStateCommand{}, 1*time.Second)
		result, err := future.Result()
		if err != nil {
			t.Fatalf("获取状态失败: %v", err)
		}

		state := result.(*TestState)
		expectedState := &TestState{
			Counter:      10, // 1+2+3+4
			RestartCount: 0,
		}
		assertStateEquals(t, expectedState, state, "快照策略测试")
	})
}

// =============================================================================
// 监管策略和重启恢复测试
// =============================================================================

func TestSupervisionAndRecovery(t *testing.T) {
	system, cleanup := createTestSystem(t)
	defer cleanup()

	repo := persistencerepos.NewMemory()

	t.Run("Actor panic后监管重启的持久化恢复", func(t *testing.T) {
		// 创建具有重启监管策略的Actor
		actor := system.ActorOf(func() vivid.Actor {
			return NewPersistentTestActor("test-supervision")
		}, func(config *vivid.ActorConfig) {
			config.WithName("supervision-test").
				WithDefaultPersistence(repo).
				WithSupervisor(vivid.SupervisorFN(func(snapshot vivid.AccidentSnapshot) {
					// 重启策略
					snapshot.Restart(snapshot.GetVictim())
				}))
		})

		// 发送一些事件建立状态
		system.Tell(actor, int64(10))
		system.Tell(actor, int64(20))
		system.Tell(actor, "important-data")
		time.Sleep(100 * time.Millisecond)

		// 验证panic前的状态
		future1 := system.Ask(actor, &GetStateCommand{}, 1*time.Second)
		result1, err := future1.Result()
		if err != nil {
			t.Fatalf("获取panic前状态失败: %v", err)
		}

		prePanicState := result1.(*TestState)
		expectedPrePanic := &TestState{
			Counter:      30,
			History:      []string{"important-data"},
			RestartCount: 0,
		}
		assertStateEquals(t, expectedPrePanic, prePanicState, "panic前状态")

		// 触发panic
		system.Tell(actor, &PanicCommand{})

		// 等待重启完成 - 通过检查RestartCount来判断
		waitForCondition(t, func() bool {
			future := system.Ask(actor, &GetStateCommand{}, 500*time.Millisecond)
			if result, err := future.Result(); err == nil {
				if state, ok := result.(*TestState); ok {
					return state.RestartCount > 0
				}
			}
			return false
		}, 3*time.Second, "等待Actor重启")

		// 验证重启后状态是否正确恢复
		future2 := system.Ask(actor, &GetStateCommand{}, 1*time.Second)
		result2, err := future2.Result()
		if err != nil {
			t.Fatalf("获取重启后状态失败: %v", err)
		}

		postRestartState := result2.(*TestState)

		// 验证状态恢复正确，且重启计数增加
		if postRestartState.Counter != 30 {
			t.Errorf("重启后Counter不匹配, 期望: 30, 实际: %d", postRestartState.Counter)
		}
		if len(postRestartState.History) != 1 || postRestartState.History[0] != "important-data" {
			t.Errorf("重启后History不匹配, 期望: [important-data], 实际: %v", postRestartState.History)
		}
		if postRestartState.RestartCount != 1 {
			t.Errorf("重启计数不匹配, 期望: 1, 实际: %d", postRestartState.RestartCount)
		}

		// 验证重启后功能正常
		system.Tell(actor, int64(5))
		system.Tell(actor, "after-restart")
		time.Sleep(100 * time.Millisecond)

		future3 := system.Ask(actor, &GetStateCommand{}, 1*time.Second)
		result3, err := future3.Result()
		if err != nil {
			t.Fatalf("获取最终状态失败: %v", err)
		}

		finalState := result3.(*TestState)
		expectedFinal := &TestState{
			Counter:      35, // 30 + 5
			History:      []string{"important-data", "after-restart"},
			RestartCount: 1,
		}
		assertStateEquals(t, expectedFinal, finalState, "重启后的最终状态")
	})
}

// =============================================================================
// 并发和边界条件测试
// =============================================================================

func TestConcurrencyAndEdgeCases(t *testing.T) {
	system, cleanup := createTestSystem(t)
	defer cleanup()

	repo := persistencerepos.NewMemory()

	t.Run("并发消息处理", func(t *testing.T) {
		actor := system.ActorOf(func() vivid.Actor {
			return NewPersistentTestActor("test-concurrent")
		}, func(config *vivid.ActorConfig) {
			config.WithName("concurrent-test").WithDefaultPersistence(repo)
		})

		// 并发发送大量消息（Actor模型保证串行处理）
		const messageCount = 100
		var wg sync.WaitGroup

		for i := 0; i < messageCount; i++ {
			wg.Add(1)
			go func(value int64) {
				defer wg.Done()
				system.Tell(actor, value)
			}(int64(i + 1))
		}

		wg.Wait()
		time.Sleep(500 * time.Millisecond) // 等待所有消息处理完成

		// 验证最终状态
		future := system.Ask(actor, &GetStateCommand{}, 1*time.Second)
		result, err := future.Result()
		if err != nil {
			t.Fatalf("获取并发测试状态失败: %v", err)
		}

		state := result.(*TestState)
		expectedSum := int64(messageCount * (messageCount + 1) / 2) // 1+2+...+100 = 5050

		if state.Counter != expectedSum {
			t.Errorf("并发测试计数器不匹配, 期望: %d, 实际: %d", expectedSum, state.Counter)
		}
	})

	t.Run("空状态处理", func(t *testing.T) {
		actor := system.ActorOf(func() vivid.Actor {
			return NewPersistentTestActor("test-empty")
		}, func(config *vivid.ActorConfig) {
			config.WithName("empty-test").WithDefaultPersistence(repo)
		})

		// 不发送任何消息，直接验证初始状态
		future := system.Ask(actor, &GetStateCommand{}, 1*time.Second)
		result, err := future.Result()
		if err != nil {
			t.Fatalf("获取空状态失败: %v", err)
		}

		state := result.(*TestState)
		expectedEmpty := &TestState{
			Counter:      0,
			History:      []string{},
			RestartCount: 0,
		}
		assertStateEquals(t, expectedEmpty, state, "空状态")
	})
}

// =============================================================================
// 性能测试
// =============================================================================

func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	system, cleanup := createTestSystem(t)
	defer cleanup()

	repo := persistencerepos.NewMemory()

	t.Run("持久化性能测试", func(t *testing.T) {
		actor := system.ActorOf(func() vivid.Actor {
			return NewPersistentTestActor("test-performance")
		}, func(config *vivid.ActorConfig) {
			config.WithName("performance-test").WithDefaultPersistence(repo)
		})

		start := time.Now()
		const eventCount = 1000

		// 发送大量事件
		for i := 0; i < eventCount; i++ {
			event := &TestEvent{
				Type:      "perf-test",
				Value:     int64(i + 1),
				Timestamp: time.Now().Format("15:04:05.000"),
			}
			system.Tell(actor, event)
		}

		// 等待处理完成
		time.Sleep(1 * time.Second)

		duration := time.Since(start)

		// 验证状态
		future := system.Ask(actor, &GetStateCommand{}, 2*time.Second)
		result, err := future.Result()
		if err != nil {
			t.Fatalf("获取性能测试状态失败: %v", err)
		}

		state := result.(*TestState)
		expectedSum := int64(eventCount * (eventCount + 1) / 2)

		if state.Counter != expectedSum {
			t.Errorf("性能测试计数器不匹配, 期望: %d, 实际: %d", expectedSum, state.Counter)
		}

		if len(state.History) != eventCount {
			t.Errorf("性能测试历史记录数量不匹配, 期望: %d, 实际: %d", eventCount, len(state.History))
		}

		t.Logf("性能测试完成: %d个事件在%v内处理完成, 平均每个事件%v",
			eventCount, duration, duration/time.Duration(eventCount))
	})
}

// =============================================================================
// 入口测试函数
// =============================================================================

func TestPersistence(t *testing.T) {
	t.Run("BasicPersistence", TestBasicPersistence)
	t.Run("SnapshotPolicy", TestSnapshotPolicy)
	t.Run("SupervisionAndRecovery", TestSupervisionAndRecovery)
	t.Run("ConcurrencyAndEdgeCases", TestConcurrencyAndEdgeCases)
	t.Run("Performance", TestPerformance)
}
