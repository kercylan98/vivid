package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type protectedTestHandler struct {
	slog.Handler
	fail   string
	derive bool
}

func (h *protectedTestHandler) Enabled(context.Context, slog.Level) bool {
	if h.fail == "enabled" {
		panic(h.fail)
	}
	return true
}

func (h *protectedTestHandler) Handle(context.Context, slog.Record) error { panic("handle") }
func (h *protectedTestHandler) WithAttrs([]slog.Attr) slog.Handler {
	if h.derive {
		return h
	}
	panic("attrs")
}

func (h *protectedTestHandler) WithGroup(string) slog.Handler {
	if h.derive {
		return h
	}
	panic("group")
}

func TestProtect_Builtin(t *testing.T) {
	for _, backend := range []string{"vivid", "slog"} {
		for _, failure := range []string{"enabled", "handle"} {
			t.Run(backend+"/"+failure, func(t *testing.T) {
				handler := &protectedTestHandler{fail: failure}
				var raw Logger = &VividLogger{handler: handler}
				if backend == "slog" {
					raw = NewSLogLogger(slog.New(handler))
				}
				logger := Protect(raw)
				require.NotPanics(t, func() {
					logger.Debug("debug")
					logger.Info("info")
					logger.Warn("warn")
					logger.Error("error")
				})
				require.Panics(t, func() { raw.Error("raw logger") })
				require.Same(t, logger, logger.With("request", 1))
				require.Same(t, logger, logger.WithGroup("worker"))
				require.Same(t, logger, Protect(logger))
				handler.derive = true
				require.NotPanics(t, func() { logger.With("request", 1).WithGroup("worker").Error("derived") })
			})
		}
	}
}

type panicOnceWriter struct {
	bytes.Buffer
	attempts int
}

func (w *panicOnceWriter) Write(data []byte) (int, error) {
	w.attempts++
	if w.attempts == 1 {
		panic("writer panic")
	}
	return w.Buffer.Write(data)
}

func TestProtect_WriterUnlock(t *testing.T) {
	for _, warning := range []bool{false, true} {
		t.Run(fmt.Sprint(warning), func(t *testing.T) {
			writer := new(panicOnceWriter)
			raw := NewLogger(WithWriterOutput(writer, OutputJSON)).(*VividLogger)
			logger := Protect(raw).With("request", 1).WithGroup("worker")
			if warning {
				// 限流诊断与普通日志共用输出锁，失败后都须允许后续输出。
				require.PanicsWithValue(t, "writer panic", func() {
					raw.handler.(*VividHandler).emitRateLimitWarning(time.Now(), 1)
				})
			} else {
				require.NotPanics(t, func() { logger.Error("first") })
			}
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				logger.Info("second")
			}()
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("Writer panic 后输出锁未释放")
			}
			require.Equal(t, 2, writer.attempts)
			require.Contains(t, writer.String(), "second")
			require.Contains(t, writer.String(), "request")
			require.Contains(t, writer.String(), "worker")
		})
	}
}

func TestProtect_Source(t *testing.T) {
	for _, derived := range []bool{false, true} {
		t.Run(fmt.Sprint(derived), func(t *testing.T) {
			var output bytes.Buffer
			logger := Protect(NewLogger(WithWriterOutput(&output, OutputJSON), WithLevel(LevelDebug)))
			if derived {
				logger = logger.With("request", 1).WithGroup("worker")
			}
			_, file, line, _ := runtime.Caller(0)
			logger.Debug("source")
			require.Contains(t, output.String(), fmt.Sprintf("%s:%d", file[strings.LastIndex(file, "/")+1:], line+1))
		})
	}
}

func TestProtect_CallSkip(t *testing.T) {
	for _, skip := range []int{0, 1} {
		var output bytes.Buffer
		raw := NewLogger(WithWriterOutput(&output, OutputJSON), WithCallSkip(skip))
		for _, logger := range []Logger{raw, Protect(raw)} {
			logger.Info("source")
		}
		lines := strings.Split(strings.TrimSpace(output.String()), "\n")
		require.Len(t, lines, 2)
		var before, after map[string]any
		require.NoError(t, json.Unmarshal([]byte(lines[0]), &before))
		require.NoError(t, json.Unmarshal([]byte(lines[1]), &after))
		require.Equal(t, before["source"], after["source"], "保护不能改变额外跳栈后的定位")
	}
}

func BenchmarkProtect_Disabled(b *testing.B) {
	for _, protected := range []bool{false, true} {
		b.Run(fmt.Sprint(protected), func(b *testing.B) {
			logger := NewSilentLogger(WithLevel(LevelError))
			if protected {
				logger = Protect(logger)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				logger.Debug("disabled")
			}
		})
	}
}

func BenchmarkProtect_Output(b *testing.B) {
	logger := Protect(NewLogger(WithWriterOutput(io.Discard, OutputJSON), WithLevel(LevelDebug)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.Info("message")
	}
}
