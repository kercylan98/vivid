package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_RestartLaunchTarget(t *testing.T) {
	for _, provider := range []bool{false, true} {
		t.Run(map[bool]string{false: "reuse", true: "provider"}[provider], func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			launched := make(chan vivid.ActorRef, 1)
			prelaunch, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			resume := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(resume)

			type childState struct {
				ref        vivid.ActorRef
				generation int
				steps      []string
			}
			var steps []string
			var restarting bool
			var generation int
			newActor := func() vivid.Actor {
				generation++
				current := generation
				return vivid.NewComplexCombinationActor(
					vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
						steps = append(steps, "restarted")
						return nil
					}),
					vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
						steps = append(steps, "prelaunch")
						if restarting {
							close(prelaunch)
							<-release
						}
						return nil
					}),
					vivid.ActorFN(func(ctx vivid.ActorContext) {
						switch ctx.Message().(type) {
						case *vivid.OnLaunch:
							steps = append(steps, "launch")
							if !restarting {
								restarting = true
								launched <- ctx.Ref()
								ctx.Failed("restart launch target")
							}
						case string:
							steps = append(steps, "message")
							ctx.Reply(childState{ctx.Ref(), current, append([]string(nil), steps...)})
						}
					}),
				)
			}
			var parentLaunches int
			parent, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case *vivid.OnLaunch:
					parentLaunches++
					if parentLaunches != 1 {
						return
					}
					var options []vivid.ActorOption
					if provider {
						options = append(options, vivid.WithActorProvider(vivid.ActorProviderFN(newActor)))
					}
					_, err := ctx.ActorOf(newActor(), options...)
					assert.NoError(t, err)
				case string:
					ctx.Reply(parentLaunches)
				}
			}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(
				func(ctx vivid.SupervisionContext) (vivid.SupervisionDecision, string) {
					return vivid.SupervisionDecisionRestart, "restart launch target"
				},
			))))
			require.NoError(t, err)
			child := receiveRetryResult(t, launched)
			receiveRetryResult(t, prelaunch)

			// 固定新实例预启动的窗口，确保业务消息已在恢复邮箱前排队。
			pending := system.Ask(child, "state", time.Second)
			resume()
			result, err := pending.Result()
			require.NoError(t, err)
			state := result.(childState)
			assert.Same(t, child, state.ref, "重启应保留原有引用")
			assert.Equal(t, map[bool]int{false: 1, true: 2}[provider], state.generation)
			assert.Equal(t, []string{"prelaunch", "launch", "restarted", "prelaunch", "launch", "message"}, state.steps,
				"重启后的启动消息必须先于排队业务消息交给子 Actor")
			result, err = system.Ask(parent, "state", time.Second).Result()
			require.NoError(t, err)
			assert.Equal(t, 1, result, "子 Actor 重启不能再次启动父 Actor")
		})
	}
}
