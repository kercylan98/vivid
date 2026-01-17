package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

var (
	_             Logger = (*SLogLogger)(nil)
	_             Logger = (*VividLogger)(nil)
	defaultLogger atomic.Pointer[Logger]
)

func init() {
	logger := NewTextLogger()
	SetDefault(logger)
}

// Logger 定义了日志记录器的抽象接口，约束了日志系统的通用操作方法规范。
//
// 该接口旨在为框架内的日志功能提供统一抽象及兼容处理，支持灵活替换、扩展底层日志实现（如 slog、zap、logrus 等）。
// 使用该接口可实现标准化的分级日志输出、结构化日志、上下文增强等能力，从而满足多样化的业务及运维需求。
//
// 主要方法说明：
//   - Debug：输出调试级别日志，适用于开发阶段的详细信息、变量值追踪等，生产环境可禁用。
//   - Info：输出常规信息级别日志，用于记录系统关键事件及正常流程跟踪。
//   - Warn：输出警告级别日志，针对潜在风险、非预期但可恢复情况进行提示。
//   - Error：输出错误级别日志，用于记录异常、故障等需重点关注的问题。
//   - With：携带额外上下文键值对，构建带有结构化上下文的子 Logger，链式传递业务标识、请求追踪等信息。
//   - WithGroup：基于分组/命名空间生成新的 Logger 实例，用于组织、区分系统子模块的日志归属，提升可读性与检索效率。
//
// 参数约定说明：
//   - message：日志主体文本，支持格式化或纯文本输出。
//   - args：变长参数，通常为结构化键值对（建议偶数，键为 string），用于丰富日志上下文。
//   - group：分组或命名空间标识，方便日志聚合与分类展示。
type Logger interface {
	// Debug 输出调试级别日志。
	// 适用于开发、调试场景下的详细信息追踪，生产环境通常默认关闭。
	Debug(message string, args ...any)

	// Info 输出信息级别日志。
	// 用于记录系统运行的关键事件、配置信息或业务正常流程节点。
	Info(message string, args ...any)

	// Warn 输出警告级别日志。
	// 主要反映潜在问题或非预期但未必致命的异常，可用于监控风险点。
	Warn(message string, args ...any)

	// Error 输出错误级别日志。
	// 用于捕获系统或业务中的严重异常、错误事件，需重点关注与定位。
	Error(message string, args ...any)

	// With 返回一个包含额外结构化上下文信息的新 Logger 实例。
	// 可链式添加业务标识、追踪信息等，便于日志聚合与定位问题。
	With(args ...any) Logger

	// WithGroup 依据指定分组/命名空间，生成新的 Logger 实例。
	// 适合大规模系统内模块化、分层化的日志归类与统一管理。
	WithGroup(group string) Logger
}

func NewSLogLogger(logger *slog.Logger) Logger {
	return &SLogLogger{logger: logger}
}

// NewLogger 创建一个使用自定义 Handler 的日志记录器。
// 若未传入任何配置项，则使用默认配置与输出目标。
func NewLogger(options ...Option) Logger {
	opts := NewHandlerConfig(options...)
	handler := NewHandler(options...)

	return &VividLogger{
		handler:  handler,
		callSkip: opts.CallSkip,
	}
}

// NewTextLogger 创建文本格式的控制台日志记录器。
// 该构造器默认启用颜色输出，可通过 WithConsoleOutput 覆盖输出细节。
func NewTextLogger(options ...Option) Logger {
	return NewLogger(append([]Option{
		WithConsoleOutput(os.Stderr, OutputText, true),
	}, options...)...)
}

// NewJSONLogger 创建 JSON 格式的控制台日志记录器。
// 适用于结构化日志采集场景，可通过 WithConsoleOutput 覆盖输出细节。
func NewJSONLogger(options ...Option) Logger {
	return NewLogger(append([]Option{
		WithConsoleOutput(os.Stderr, OutputJSON, false),
	}, options...)...)
}

// NewFileLogger 创建文件输出的日志记录器。
// 可通过 RotationOptions 配置轮转与保留策略。
func NewFileLogger(path string, rotation RotationOptions, options ...Option) Logger {
	return NewLogger(append([]Option{
		WithFileOutput(FileOutputOptions{
			Path:     path,
			Rotation: rotation,
			Format:   OutputText,
		}),
	}, options...)...)
}

// NewJSONFileLogger 创建 JSON 格式的文件日志记录器。
// 可通过 RotationOptions 配置轮转与保留策略。
func NewJSONFileLogger(path string, rotation RotationOptions, options ...Option) Logger {
	return NewLogger(append([]Option{
		WithFileOutput(FileOutputOptions{
			Path:     path,
			Rotation: rotation,
			Format:   OutputJSON,
		}),
	}, options...)...)
}

// NewSilentLogger 创建静默日志记录器。
// 该构造器会丢弃所有日志输出，适用于测试或禁用日志的场景。
func NewSilentLogger(options ...Option) Logger {
	return NewLogger(append([]Option{
		WithOutput(OutputOptions{
			Writer: io.Discard,
			Format: OutputText,
			Color:  false,
		}),
	}, options...)...)
}

// SetDefault 设置默认的日志记录器。
// 若 logger 为 nil，则使用 slog.Default() 作为默认日志记录器。
func SetDefault(logger Logger) {
	if logger == nil {
		logger = NewLogger()
	}
	defaultLogger.Store(&logger)
}

// GetDefault 获取默认的日志记录器。
func GetDefault() Logger {
	return *defaultLogger.Load()
}

type SLogLogger struct {
	logger *slog.Logger
}

func (s *SLogLogger) Debug(message string, args ...any) {
	s.logger.Debug(message, args...)
}

func (s *SLogLogger) Error(message string, args ...any) {
	s.logger.Error(message, args...)
}

func (s *SLogLogger) Info(message string, args ...any) {
	s.logger.Info(message, args...)
}

func (s *SLogLogger) Warn(message string, args ...any) {
	s.logger.Warn(message, args...)
}

func (s *SLogLogger) With(args ...any) Logger {
	return &SLogLogger{logger: s.logger.With(args...)}
}

func (s *SLogLogger) WithGroup(group string) Logger {
	return &SLogLogger{logger: s.logger.WithGroup(group)}
}

// VividLogger 基于可配置的 Handler 输出日志，支持调用栈跳过与增强能力。
type VividLogger struct {
	handler  slog.Handler
	callSkip int
}

func (v *VividLogger) Debug(message string, args ...any) {
	v.log(slog.LevelDebug, message, args...)
}

func (v *VividLogger) Error(message string, args ...any) {
	v.log(slog.LevelError, message, args...)
}

func (v *VividLogger) Info(message string, args ...any) {
	v.log(slog.LevelInfo, message, args...)
}

func (v *VividLogger) Warn(message string, args ...any) {
	v.log(slog.LevelWarn, message, args...)
}

func (v *VividLogger) With(args ...any) Logger {
	attrs := attrsFromArgs(args...)
	return &VividLogger{
		handler:  v.handler.WithAttrs(attrs),
		callSkip: v.callSkip,
	}
}

func (v *VividLogger) WithGroup(group string) Logger {
	return &VividLogger{
		handler:  v.handler.WithGroup(group),
		callSkip: v.callSkip,
	}
}

func (v *VividLogger) log(level slog.Level, message string, args ...any) {
	if v.handler == nil {
		return
	}

	if !v.handler.Enabled(context.Background(), level) {
		return
	}

	record := slog.NewRecord(time.Now(), level, message, 0)
	record.PC = callerPC(4 + v.callSkip)
	record.Add(args...)
	_ = v.handler.Handle(context.Background(), record)
}

func callerPC(skip int) uintptr {
	var pcs [1]uintptr
	if runtime.Callers(skip, pcs[:]) == 0 {
		return 0
	}
	return pcs[0] - 1
}

func attrsFromArgs(args ...any) []slog.Attr {
	record := slog.NewRecord(time.Time{}, 0, "", 0)
	record.Add(args...)

	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	return attrs
}
