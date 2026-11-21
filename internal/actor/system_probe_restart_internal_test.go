package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestSystem_ProbeRestartAvailability(t *testing.T) {
	for _, stage := range []string{"restarted", "prelaunch"} {
		for _, outcome := range []string{"success", "error", "panic"} {
			t.Run(stage+"/"+outcome, func(t *testing.T) {
				newSystem := func() *System {
					system := NewTestSystem(t,
						vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
						vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
					)
					t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
					return system.System
				}
				system, remote := newSystem(), newSystem()
				entered, release, launched := make(chan struct{}), make(chan struct{}), make(chan struct{}, 2)
				var releaseOnce sync.Once
				unblock := func() { releaseOnce.Do(func() { close(release) }) }
				defer unblock()
				hook := func() error {
					close(entered)
					<-release
					switch outcome {
					case "error":
						return vivid.ErrorIllegalArgument.WithMessage("probe restart failure")
					case "panic":
						panic("probe restart failure")
					}
					return nil
				}
				var prelaunches int
				ref, err := system.ActorOf(vivid.NewComplexCombinationActor(
					vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
						prelaunches++
						if stage == "prelaunch" && prelaunches > 1 {
							return hook()
						}
						return nil
					}),
					vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
						if stage == "restarted" {
							return hook()
						}
						return nil
					}),
					vivid.ActorFN(func(ctx vivid.ActorContext) {
						switch ctx.Message().(type) {
						case *vivid.OnLaunch:
							launched <- struct{}{}
						case string:
							ctx.Reply("alive")
						}
					}),
				))
				require.NoError(t, err)
				receiveRetryResult(t, launched)
				check := func(available bool) {
					t.Helper()
					for _, source := range []*System{system, remote} {
						heartbeat, err := source.Probe(ref.Clone()).Result()
						require.NoError(t, err)
						require.True(t, ref.Equals(heartbeat.Ref))
						require.Equal(t, available, heartbeat.Available)
					}
				}
				check(true)
				system.AdvancedTell(true, ref, &RestartMessage{Reason: "probe restart"})
				receiveRetryResult(t, entered)
				// 初始化仍被阻塞时，本地与真实 TCP 探测都应直接返回不可用。
				check(false)
				unblock()
				if outcome == "success" {
					receiveRetryResult(t, launched)
					result, err := system.Ask(ref, "business", time.Second).Result()
					require.NoError(t, err)
					require.Equal(t, "alive", result)
				} else {
					// 框架 Ping 排在重启结束后，确认已进入僵尸状态再检查 Probe。
					_, err := system.Ask(ref, new(messages.PingMessage), time.Second).Result()
					require.NoError(t, err)
					require.Empty(t, launched)
				}
				check(outcome == "success")
				found, err := system.FindActor(ref.String())
				require.NoError(t, err, "探测不可用不能提前移除待清理的引用")
				require.True(t, ref.Equals(found))
			})
		}
	}
}
