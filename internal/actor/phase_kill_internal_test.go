package actor

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestContext_PhaseKillPausedMailbox(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "timeout"}[timeout], func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			completed, launched, started, stopped := make(chan struct{}), make(chan *Context, 1), make(chan struct{}, 1), make(chan struct{}, 1)
			var messages atomic.Int32
			behavior := vivid.Behavior(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					launched <- ctx.(*Context)
				case *vivid.OnKill:
					started <- struct{}{}
				case *vivid.OnKilled:
					if message.Ref.Equals(ctx.Ref()) {
						stopped <- struct{}{}
					}
				case string:
					messages.Add(1)
				}
			})
			wait := time.Second
			if timeout {
				wait = 25 * time.Millisecond
			}
			ref, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				return ctx.WithPhaseKill(completed, wait, behavior)
			}, vivid.ActorFN(behavior)))
			require.NoError(t, err)
			ctx := receiveRetryResult(t, launched)
			ctx.mailbox.Pause()
			t.Cleanup(ctx.mailbox.Resume)
			system.Tell(ref, "paused message")
			system.Kill(ref, false, "paused phase kill")
			receiveRetryResult(t, started)
			if !timeout {
				close(completed)
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("普通邮箱暂停时，多阶段终止的完成或超时通知必须仍可处理")
			}
			require.True(t, ctx.mailbox.IsPaused(), "完成通知不能恢复普通消息消费")
			require.Zero(t, messages.Load())
		})
	}
}

func TestContext_PhaseKillStaleCompletionAfterRestart(t *testing.T) {
	for _, register := range []bool{true, false} {
		t.Run(map[bool]string{true: "registered", false: "unregistered"}[register], func(t *testing.T) {
			testPhaseKillStaleCompletionAfterRestart(t, register)
		})
	}
}

func testPhaseKillStaleCompletionAfterRestart(t *testing.T, register bool) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	phases, launched, started, stopped := make(chan *phaseKill, 2), make(chan *Context, 1), make(chan struct{}, 4), make(chan struct{}, 4)
	behavior := vivid.Behavior(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			launched <- ctx.(*Context)
		case *vivid.OnKill:
			started <- struct{}{}
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				stopped <- struct{}{}
			}
		case func(vivid.ActorContext):
			message(ctx)
		}
	})
	var signals []chan struct{}
	ref, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		if len(signals) > 0 && !register {
			phases <- ctx.(*Context).phaseKill
			return nil
		}
		completed := make(chan struct{})
		signals = append(signals, completed)
		if err := ctx.WithPhaseKill(completed, time.Second, behavior); err != nil {
			return err
		}
		phases <- ctx.(*Context).phaseKill
		return nil
	}, vivid.ActorFN(behavior)))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, launched)
	previous := receiveRetryResult(t, phases)
	t.Cleanup(ctx.mailbox.Resume)

	// 固定重启前的暂停窗口，并保留旧阶段通知，供重启后重放。
	ctx.mailbox.Pause()
	ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, &RestartMessage{Reason: "phase restart"}))
	receiveRetryResult(t, started)
	close(signals[0])
	receiveRetryResult(t, stopped)
	current := receiveRetryResult(t, phases)
	if register {
		require.NotSame(t, previous, current)
	} else {
		require.Nil(t, current, "重启后未重新注册时，不能继承旧实例的终止阶段")
	}

	check := func(expected int32) {
		t.Helper()
		result := make(chan bool, 1)
		ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, previous))
		ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, previous))
		ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, func(actorContext vivid.ActorContext) {
			c := actorContext.(*Context)
			result <- atomic.LoadInt32(&c.state) == expected && c.phaseKill == current && (current == nil || !current.completed)
		}))
		require.True(t, receiveRetryResult(t, result), "旧阶段通知不能改变新阶段状态")
		require.Empty(t, stopped)
		require.Empty(t, started)
	}
	// 新阶段尚未开始时收到的内部通知也应忽略。
	if current != nil {
		ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, current))
	}
	check(running)
	ctx.mailbox.Pause()
	system.Kill(ref, false, "new phase kill")
	receiveRetryResult(t, started)
	if register {
		check(killing)
		close(signals[1])
	}
	receiveRetryResult(t, stopped)
	require.True(t, ctx.mailbox.IsPaused())
	require.Empty(t, started)
}

func TestContext_PhaseKillCompletionWaitsForChildren(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	completed, launched, childStarted, releaseChild, stopped := make(chan struct{}), make(chan *Context, 1), make(chan struct{}), make(chan struct{}), make(chan struct{}, 2)
	defer close(releaseChild)
	behavior := vivid.Behavior(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			_, err := ctx.ActorOf(vivid.ActorFN(func(child vivid.ActorContext) {
				if _, ok := child.Message().(*vivid.OnKill); ok {
					close(childStarted)
					<-releaseChild
				}
			}))
			if err != nil {
				panic(err)
			}
			launched <- ctx.(*Context)
		case *vivid.OnKill:
			for _, child := range ctx.Children() {
				ctx.Kill(child, false, "phase child stop")
			}
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				stopped <- struct{}{}
			}
		case func(vivid.ActorContext):
			message(ctx)
		}
	})
	ref, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		return ctx.WithPhaseKill(completed, time.Second, behavior)
	}, vivid.ActorFN(behavior)))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, launched)
	ctx.mailbox.Pause()
	t.Cleanup(ctx.mailbox.Resume)
	system.Kill(ref, false, "phase parent stop")
	receiveRetryResult(t, childStarted)
	close(completed)
	// 等待实际完成通知已被处理，再重放相同通知，固定父节点仍在等待子节点的窗口。
	require.Eventually(t, func() bool {
		result := make(chan bool, 1)
		ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, func(actorContext vivid.ActorContext) {
			c := actorContext.(*Context)
			result <- c.phaseKill.completed
		}))
		return receiveRetryResult(t, result)
	}, time.Second, time.Millisecond)
	result := make(chan bool, 1)
	ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, ctx.phaseKill))
	ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, ctx.phaseKill))
	ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, ref, ref, func(actorContext vivid.ActorContext) {
		c := actorContext.(*Context)
		result <- atomic.LoadInt32(&c.state) == killing && c.phaseKill.envelope == nil && c.Children().Len() == 1
	}))
	require.True(t, receiveRetryResult(t, result))
	require.Empty(t, stopped, "完成信号不能越过尚未退出的子 Actor")

	// 解除子 Actor 的停止阻塞后，父 Actor 无需恢复普通邮箱即可结束。
	releaseChild <- struct{}{}
	receiveRetryResult(t, stopped)
	require.True(t, ctx.mailbox.IsPaused())
	require.Empty(t, stopped)
}
