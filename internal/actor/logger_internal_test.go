package actor

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type loggerProbe struct {
	write func(level, message string, args []any)
	with  func(args []any) log.Logger
	group func(group string) log.Logger
}

func (l *loggerProbe) Debug(message string, args ...any) { l.output("debug", message, args) }
func (l *loggerProbe) Info(message string, args ...any)  { l.output("info", message, args) }
func (l *loggerProbe) Warn(message string, args ...any)  { l.output("warn", message, args) }
func (l *loggerProbe) Error(message string, args ...any) { l.output("error", message, args) }

func (l *loggerProbe) output(level, message string, args []any) {
	if l.write != nil {
		l.write(level, message, args)
	}
}

func (l *loggerProbe) With(args ...any) log.Logger {
	if l.with != nil {
		return l.with(args)
	}
	return l
}

func (l *loggerProbe) WithGroup(group string) log.Logger {
	if l.group != nil {
		return l.group(group)
	}
	return l
}

func TestLogger_Forwarding(t *testing.T) {
	var levels []string
	argument := new(int)
	child := &loggerProbe{write: func(level, message string, args []any) {
		levels = append(levels, level)
		require.Equal(t, "message", message)
		require.Equal(t, []any{"key", argument}, args)
	}}
	raw := &loggerProbe{
		with: func(args []any) log.Logger {
			require.Equal(t, []any{"request", argument}, args)
			return child
		},
		group: func(group string) log.Logger {
			require.Equal(t, "worker", group)
			return child
		},
	}
	system := NewSystem(vivid.WithActorSystemLogger(raw))
	logger := system.Logger().With("request", argument)
	logger.Debug("message", "key", argument)
	logger.Info("message", "key", argument)
	logger.Warn("message", "key", argument)
	logger.Error("message", "key", argument)
	system.Logger().WithGroup("worker").Info("message", "key", argument)
	require.Equal(t, []string{"debug", "info", "warn", "error", "info"}, levels)

	// 派生日志器也受保护；框架不修改用户持有的原始日志器。
	child.write = func(string, string, []any) { panic("child logger panic") }
	require.NotPanics(t, func() { logger.Error("fault") })
	require.NotPanics(t, func() { system.Logger().WithGroup("worker").Debug("fault") })
	require.PanicsWithValue(t, "child logger panic", func() { child.Error("fault") })
}

func TestLogger_Panic(t *testing.T) {
	for _, fault := range []any{"logger panic", nil} {
		var attempts int
		fail := func() { attempts++; panic(fault) }
		raw := &loggerProbe{
			write: func(string, string, []any) { fail() },
			with:  func([]any) log.Logger { fail(); return nil },
			group: func(string) log.Logger { fail(); return nil },
		}
		logger := NewSystem(vivid.WithActorSystemLogger(raw)).Logger()
		require.NotPanics(t, func() {
			logger.Debug("debug")
			logger.Info("info")
			logger.Warn("warn")
			logger.Error("error")
			require.Same(t, logger, logger.With("request", 1))
			require.Same(t, logger, logger.WithGroup("worker"))
		})
		require.Equal(t, 6, attempts, "日志器失败后不重复输出或递归记录")
	}
	logger := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{
		with:  func([]any) log.Logger { return nil },
		group: func(string) log.Logger { return nil },
	})).Logger()
	require.Same(t, logger, logger.With("request", 1))
	require.Same(t, logger, logger.WithGroup("worker"))
}

func TestLogger_Lifecycle(t *testing.T) {
	const scenarioEnv = "VIVID_LOGGER_LIFECYCLE"
	if os.Getenv(scenarioEnv) == "" {
		// 日志回归可能发生在邮箱或关闭协程，使用独立进程保留崩溃证据。
		ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLogger_Lifecycle$", "-test.timeout=20s")
		command.Env = append(os.Environ(), scenarioEnv+"=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}
	logger := &loggerProbe{
		write: func(string, string, []any) { panic("output panic") },
		with:  func([]any) log.Logger { panic("with panic") },
		group: func(string) log.Logger { panic("group panic") },
	}
	system := NewTestSystem(t, vivid.WithActorSystemLogger(logger), vivid.WithActorSystemRemoting(admissionSystemAddress(t)))
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	target, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
	require.NoError(t, err)
	ready, killed := make(chan *Context, 1), make(chan struct{}, 1)
	stream := system.eventStream.(*eventStream)
	system.eventStream = &startupEventStream{EventStream: stream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() == "/worker" {
			killed <- struct{}{}
		}
	}}
	var pending vivid.ScheduledTask
	var request, task vivid.Future[vivid.Message]
	release, finished := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, 0)
			var scheduleErr error
			pending, scheduleErr = ctx.Scheduler().Once(target, time.Hour, "pending")
			if scheduleErr != nil {
				panic(scheduleErr)
			}
			request = ctx.Ask(target, "pending", 0)
			task = ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
				<-release
				defer close(finished)
				return "late result", nil
			}))
			ready <- ctx.(*Context)
		case *vivid.OnKill:
			panic("on kill panic")
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				ctx.EventStream().Subscribe(ctx, "")
				panic("on killed panic")
			}
		}
	}), vivid.WithActorName("worker"), vivid.WithActorLogger(logger))
	require.NoError(t, err)
	owner := receiveRetryResult(t, ready)
	system.Kill(ref, false)
	receiveRetryResult(t, killed)
	requireEventStreamSubscriptions(t, stream, ref)
	require.False(t, pending.Exists())
	require.ErrorIs(t, request.Wait(), vivid.ErrorActorDeaded)
	require.ErrorIs(t, task.Wait(), vivid.ErrorActorDeaded)
	requireEventStreamRejected(t, stream, owner)
	_, err = system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}), vivid.WithActorName("worker"))
	require.NoError(t, err, "日志 panic 不应遗留名称或路由")
	once.Do(func() { close(release) })
	receiveRetryResult(t, finished)
	require.ErrorIs(t, task.Wait(), vivid.ErrorActorDeaded)

	for _, failure := range []bool{false, true} {
		launched := make(chan *Context, 2)
		actor := vivid.NewComplexCombinationActor(
			vivid.NewPreRestartActor(func(vivid.RestartContext) error { panic("pre restart panic") }),
			vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
				ctx.(*Context).EventStream().Subscribe(ctx, true)
				if failure {
					return context.Canceled
				}
				return nil
			}),
			vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					ctx.EventStream().Subscribe(ctx, 0)
					launched <- ctx.(*Context)
				case *vivid.OnKill:
					panic("restart kill panic")
				case *vivid.OnKilled:
					if message.Ref.Equals(ctx.Ref()) {
						panic("restart killed panic")
					}
				case string:
					ctx.Reply("alive")
				}
			}),
		)
		ref, err := system.ActorOf(actor)
		require.NoError(t, err)
		owner := receiveRetryResult(t, launched)
		system.AdvancedTell(true, ref, &RestartMessage{Reason: "logger panic"})
		_, err = system.Ask(ref, new(messages.PingMessage), time.Second).Result()
		require.NoError(t, err)
		if failure {
			require.False(t, system.HeartbeatProbe(ref).Available)
			requireEventStreamSubscriptions(t, stream, ref)
			requireEventStreamRejected(t, stream, owner)
		} else {
			receiveRetryResult(t, launched)
			result, err := system.Ask(ref, "business", time.Second).Result()
			require.NoError(t, err)
			require.Equal(t, "alive", result)
			requireEventStreamSubscriptions(t, stream, ref, 0, true)
		}
	}
	require.NoError(t, system.Stop(time.Second))
	require.Empty(t, system.futureAgents)
	require.Empty(t, system.initializingActors)
	require.Empty(t, stream.subscriberTypes)
}

func BenchmarkLogger_Debug(b *testing.B) {
	system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		system.Logger().Debug("message")
	}
}
