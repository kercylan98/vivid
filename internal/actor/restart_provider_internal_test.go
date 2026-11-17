package actor

import (
	"context"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restartNilActor 的 nil 接收方也能处理消息，避免由方法内部 panic 掩盖无效实例。
type restartNilActor struct{}

func (*restartNilActor) OnReceive(ctx vivid.ActorContext) {
	if _, ok := ctx.Message().(string); ok {
		ctx.Reply("invalid actor")
	}
}

type restartValueActor struct{}

func (restartValueActor) OnReceive(ctx vivid.ActorContext) {
	if _, ok := ctx.Message().(string); ok {
		ctx.Reply(ctx.Ref())
	}
}

func TestContext_RestartProviderValue(t *testing.T) {
	for name, actor := range map[string]vivid.Actor{
		"value":    restartValueActor{},
		"function": vivid.ActorFN(restartValueActor{}.OnReceive),
	} {
		t.Run(name, func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(string); ok {
					ctx.Reply("old actor")
				}
			}), vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return actor })))
			require.NoError(t, err)
			system.AdvancedTell(true, ref, &RestartMessage{Reason: "valid provider"})
			result, err := system.Ask(ref, "state", time.Second).Result()
			require.NoError(t, err)
			require.Same(t, ref, result, "有效的值类型和函数 Actor 仍应完成重启")
		})
	}
}

func TestContext_RestartProviderFailure(t *testing.T) {
	const scenarioEnv = "VIVID_RESTART_PROVIDER_FAILURE"
	if scenario := os.Getenv(scenarioEnv); scenario != "" {
		for _, stop := range []bool{false, true} {
			t.Run(map[bool]string{false: "kill", true: "stop"}[stop], func(t *testing.T) {
				testRestartProviderFailure(t, scenario, stop)
			})
		}
		return
	}
	// Provider 的 panic 可能直接退出进程，因此每种失败在独立子进程中验证。
	for _, scenario := range []string{"panic", "panic_nil", "nil", "typed_nil", "nil_function"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestContext_RestartProviderFailure$", "-test.timeout=10s")
			command.Env = append(os.Environ(), scenarioEnv+"="+scenario)
			output, err := command.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func testRestartProviderFailure(t *testing.T, scenario string, stop bool) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))))
	var stopped bool
	t.Cleanup(func() {
		if !stopped {
			require.NoError(t, system.Stop(time.Second))
		}
	})
	launched, provided, died := make(chan *Context, 1), make(chan struct{}), make(chan vivid.ActorRef, 1)
	var calls, handled, launches, restarted, prelaunches, restartHooks atomic.Int32
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorRestartedEvent); ok && event.ActorRef.GetPath() == "/parent/child" {
			restarted.Add(1)
		}
	}}
	require.NoError(t, system.Start())
	old := vivid.NewComplexCombinationActor(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		prelaunches.Add(1)
		return nil
	}), vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
		restartHooks.Add(1)
		return nil
	}), vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			launches.Add(1)
			launched <- ctx.(*Context)
		case string:
			if message == "fail" {
				ctx.Failed("provider failure")
			}
			handled.Add(1)
			ctx.Reply("handled")
		}
	}))
	provider := vivid.ActorProviderFN(func() vivid.Actor {
		calls.Add(1)
		close(provided)
		switch scenario {
		case "panic":
			panic("provider panic")
		case "panic_nil":
			panic(nil)
		case "typed_nil":
			return (*restartNilActor)(nil)
		case "nil_function":
			return vivid.ActorFN(nil)
		default:
			return nil
		}
	})
	parent, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			_, err := ctx.ActorOf(old, vivid.WithActorName("child"), vivid.WithActorProvider(provider))
			assert.NoError(t, err)
		case *vivid.OnKilled:
			if !message.Ref.Equals(ctx.Ref()) {
				died <- message.Ref
			}
		case string:
			ctx.Reply("alive")
		}
	}), vivid.WithActorName("parent"), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(
		vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (vivid.SupervisionDecision, string) {
			return vivid.SupervisionDecisionRestart, "provider failure"
		}),
	)))
	require.NoError(t, err)
	child := receiveRetryResult(t, launched)
	system.Tell(child.Ref(), "fail")
	receiveRetryResult(t, provided)
	// Ping 由框架处理，在同一邮箱中排在重启结束之后，避免并发读取重启中间状态。
	_, err = system.Ask(child.Ref(), new(messages.PingMessage), time.Second).Result()
	require.NoError(t, err)
	require.True(t, child.zombie, "Provider 失败应进入重启失败状态")
	require.Equal(t, killed, atomic.LoadInt32(&child.state))
	require.Same(t, old, child.actor, "无效实例不能替换旧 Actor")
	require.False(t, child.mailbox.IsPaused(), "失败后仍须排空邮箱并处理终止请求")
	_, err = system.Ask(child.Ref(), "business", 20*time.Millisecond).Result()
	require.ErrorIs(t, err, vivid.ErrorFutureTimeout)
	_, err = system.Ask(child.Ref(), new(messages.PingMessage), time.Second).Result()
	require.NoError(t, err)
	require.Zero(t, handled.Load(), "不能恢复旧业务行为")
	require.Zero(t, restarted.Load(), "失败不能发布重启成功事件")
	require.EqualValues(t, 1, prelaunches.Load(), "Provider 失败不能再次执行旧实例的预启动")
	require.Zero(t, restartHooks.Load(), "Provider 失败不能执行旧实例的重启后钩子")
	require.Empty(t, died, "尚未显式终止时保留僵尸引用")
	result, err := system.Ask(parent, "state", time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, "alive", result, "Provider 失败不能影响父 Actor")
	if stop {
		require.NoError(t, system.Stop(time.Second))
		stopped = true
	} else {
		system.Kill(child.Ref(), false)
		require.True(t, child.Ref().Equals(receiveRetryResult(t, died)))
	}
	_, err = system.FindActor(child.Ref().String())
	require.ErrorIs(t, err, vivid.ErrorNotFound)
	require.EqualValues(t, 1, calls.Load(), "失败后不自动重试 Provider")
	require.EqualValues(t, 1, launches.Load(), "失败后不发送启动消息")
}
