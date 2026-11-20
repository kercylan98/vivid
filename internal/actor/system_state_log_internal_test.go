package actor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

func TestSystem_StateLogCallbacks(t *testing.T) {
	const scenarioEnv = "VIVID_SYSTEM_STATE_LOG_CALLBACKS"
	if os.Getenv(scenarioEnv) == "" {
		// 锁回归可能阻塞启动与关闭，使用独立进程避免挂住整个套件。
		ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSystem_StateLogCallbacks$", "-test.timeout=20s")
		command.Env = append(os.Environ(), scenarioEnv+"=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}
	type outcome struct{ create, transition error }
	for _, scenario := range []struct {
		phase, method, message string
		outer, create, inner   error
	}{
		{"ready", "stop", "actor system not started", vivid.ErrorActorSystemNotStarted, vivid.ErrorActorSystemNotStarted, nil},
		{"starting", "start", "actor system is starting", vivid.ErrorActorSystemAlreadyStarted, vivid.ErrorActorSystemNotStarted, vivid.ErrorActorSystemStopFailed},
		{"start", "start", "actor system already started", vivid.ErrorActorSystemAlreadyStarted, nil, nil},
		{"stopping", "start", "actor system already stopped", vivid.ErrorActorSystemAlreadyStopped, vivid.ErrorActorSystemStopped, vivid.ErrorActorSystemStopFailed},
		{"stop", "start", "actor system already stopped", vivid.ErrorActorSystemAlreadyStopped, vivid.ErrorActorSystemStopped, vivid.ErrorActorSystemAlreadyStopped},
		{"stop", "stop", "actor system already stopped", vivid.ErrorActorSystemAlreadyStopped, vivid.ErrorActorSystemStopped, vivid.ErrorActorSystemAlreadyStopped},
	} {
		for _, mode := range []string{"reentry", "blocked"} {
			t.Run(scenario.phase+"/"+scenario.method+"/"+mode, func(t *testing.T) {
				entered, release := make(chan struct{}), make(chan struct{})
				startupEntered, startupRelease := make(chan struct{}), make(chan struct{})
				killEntered, killRelease := make(chan struct{}), make(chan struct{})
				var logOnce, startupOnce, killOnce sync.Once
				var enabled, fired atomic.Bool
				observed := make(chan outcome, 1)
				var queryAndTransition func() outcome
				logger := &loggerProbe{write: func(level, message string, _ []any) {
					if level != "warn" || message != scenario.message || !enabled.Load() || !fired.CompareAndSwap(false, true) {
						return
					}
					close(entered)
					if mode == "blocked" {
						<-release
					} else {
						observed <- queryAndTransition()
					}
				}}
				options := []vivid.ActorSystemOption{vivid.WithActorSystemLogger(logger)}
				if scenario.phase == "starting" {
					options = append(options, vivid.WithActorSystemMessageRegister(vivid.MessageRegisterFN(func(vivid.MessageRegistry) {
						close(startupEntered)
						<-startupRelease
					})))
				}
				system := NewSystem(options...)
				t.Cleanup(func() {
					enabled.Store(false)
					logOnce.Do(func() { close(release) })
					startupOnce.Do(func() { close(startupRelease) })
					killOnce.Do(func() { close(killRelease) })
					_ = system.Stop(time.Second)
				})
				var firstStart, firstStop chan error
				switch scenario.phase {
				case "starting":
					firstStart = make(chan error, 1)
					go func() { firstStart <- system.Start() }()
					receiveRetryResult(t, startupEntered)
				case "start", "stopping", "stop":
					require.NoError(t, system.Start())
					if scenario.phase == "stopping" {
						_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
							if _, ok := ctx.Message().(*vivid.OnKill); ok {
								close(killEntered)
								<-killRelease
							}
						}))
						require.NoError(t, err)
						firstStop = make(chan error, 1)
						go func() { firstStop <- system.Stop(0) }()
						receiveRetryResult(t, killEntered)
					} else if scenario.phase == "stop" {
						require.NoError(t, system.Stop(time.Second))
					}
				}
				queryAndTransition = func() outcome {
					_, createErr := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
					var transitionErr error
					switch scenario.phase {
					case "ready":
						transitionErr = system.Start()
					case "starting", "stopping":
						transitionErr = system.Stop(10 * time.Millisecond)
					case "start":
						transitionErr = system.Stop(time.Second)
					case "stop":
						if scenario.method == "start" {
							transitionErr = system.Stop()
						} else {
							transitionErr = system.Start()
						}
					}
					return outcome{createErr, transitionErr}
				}
				enabled.Store(true)
				result := make(chan error, 1)
				go func() {
					if scenario.method == "start" {
						result <- system.Start()
					} else {
						result <- system.Stop(time.Second)
					}
				}()
				receiveRetryResult(t, entered)
				if mode == "blocked" {
					// 日志尚未返回时，其他调用也应取得状态锁并完成。
					go func() { observed <- queryAndTransition() }()
				}
				state := receiveRetryResult(t, observed)
				require.ErrorIs(t, state.create, scenario.create)
				require.ErrorIs(t, state.transition, scenario.inner)
				if scenario.phase == "starting" || scenario.phase == "stopping" {
					require.ErrorIs(t, state.transition, context.DeadlineExceeded)
				}
				logOnce.Do(func() { close(release) })
				require.ErrorIs(t, receiveRetryResult(t, result), scenario.outer, "回调改变状态不能覆盖外层调用的判定")
				startupOnce.Do(func() { close(startupRelease) })
				killOnce.Do(func() { close(killRelease) })
				if firstStart != nil {
					require.ErrorIs(t, receiveRetryResult(t, firstStart), vivid.ErrorActorSystemStopped)
					receiveRetryResult(t, system.stopCompleted)
				}
				if firstStop != nil {
					require.NoError(t, receiveRetryResult(t, firstStop))
				}
				stopErr := system.Stop(time.Second)
				require.True(t, stopErr == nil || errors.Is(stopErr, vivid.ErrorActorSystemAlreadyStopped), "%v", stopErr)
				require.True(t, system.requestsStopped())
			})
		}
	}
}

func TestSystem_StateLogQuietStop(t *testing.T) {
	var warnings atomic.Int32
	system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{write: func(level, _ string, _ []any) {
		if level == "warn" {
			warnings.Add(1)
		}
	}}))
	require.ErrorIs(t, system.stop(false), vivid.ErrorActorSystemNotStarted)
	require.NoError(t, system.Start())
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	require.NoError(t, system.Stop(time.Second))
	require.ErrorIs(t, system.stop(false), vivid.ErrorActorSystemAlreadyStopped)
	require.Zero(t, warnings.Load(), "内部静默停止不记录状态提示")
}

func BenchmarkSystem_StateRejection(b *testing.B) {
	for _, scenario := range []string{"start", "start_parallel", "stop_ready", "stop_finished"} {
		b.Run(scenario, func(b *testing.B) {
			system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
			if scenario != "stop_ready" {
				require.NoError(b, system.Start())
				b.Cleanup(func() { _ = system.Stop(time.Second) })
			}
			if scenario == "stop_finished" {
				require.NoError(b, system.Stop(time.Second))
			}
			check := func() {
				var err, expected error
				if scenario == "start" || scenario == "start_parallel" {
					err, expected = system.Start(), vivid.ErrorActorSystemAlreadyStarted
				} else {
					err, expected = system.Stop(), vivid.ErrorActorSystemAlreadyStopped
					if scenario == "stop_ready" {
						expected = vivid.ErrorActorSystemNotStarted
					}
				}
				if err != expected {
					b.Errorf("expected %v, got %v", expected, err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			if scenario == "start_parallel" {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						check()
					}
				})
			} else {
				for b.Loop() {
					check()
				}
			}
		})
	}
}

func BenchmarkSystem_StartStop(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
		if err := system.Start(); err != nil {
			b.Fatal(err)
		}
		if err := system.Stop(time.Second); err != nil {
			b.Fatal(err)
		}
	}
}
