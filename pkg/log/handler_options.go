package log

import (
	"io"
	"log/slog"
	"os"
	"time"
)

// Option 定义日志 Handler 的配置项函数类型。
// 使用一组 Option 可灵活组合配置日志输出方式、轮转策略、格式化行为等。
type Option func(options *HandlerConfig)

// OutputFormat 定义日志输出格式类型。
type OutputFormat int

const (
	// OutputText 表示文本格式输出。
	OutputText OutputFormat = iota

	// OutputJSON 表示 JSON 结构化格式输出。
	OutputJSON
)

func (f OutputFormat) String() string {
	switch f {
	case OutputJSON:
		return "json"
	default:
		return "text"
	}
}

// RotationPolicy 定义日志轮转策略类型。
type RotationPolicy int

const (
	// RotationNone 表示不启用轮转。
	RotationNone RotationPolicy = iota

	// RotationBySize 表示按文件大小轮转。
	RotationBySize

	// RotationByTime 表示按时间轮转。
	RotationByTime

	// RotationBySizeAndTime 表示按大小与时间组合轮转。
	RotationBySizeAndTime
)

// RotationOptions 定义日志轮转与保留策略配置。
// 其中 MaxSize/Interval/MaxBackups/MaxAge/MaxTotalSize 均为可选项，未设置时不生效。
type RotationOptions struct {
	// Policy 指定轮转策略类型。
	Policy RotationPolicy

	// MaxSize 指定单个日志文件的最大字节数。
	MaxSize int64

	// Interval 指定按时间轮转的周期。
	Interval time.Duration

	// MaxBackups 指定最大保留文件数量。
	MaxBackups int

	// MaxAge 指定最大保留天数或时长（超过即清理）。
	MaxAge time.Duration

	// MaxTotalSize 指定所有保留日志文件的总大小上限。
	MaxTotalSize int64
}

// FileOutputOptions 定义文件输出与轮转配置。
type FileOutputOptions struct {
	// Path 指定日志文件路径。
	Path string

	// TimeFormat 指定时间格式，空值时使用全局默认时间格式。
	TimeFormat string

	// EnableSource 指定是否输出 source 信息，nil 表示使用全局默认设置。
	EnableSource *bool

	// EnableErrorStack 指定是否输出 error 堆栈，nil 表示使用全局默认设置。
	EnableErrorStack *bool

	// Rotation 指定轮转与保留策略。
	Rotation RotationOptions

	// Format 指定文件输出格式。
	Format OutputFormat

	// Append 指定是否以追加方式写入已有日志文件。
	Append bool

	// Color 指定是否启用颜色输出（通常文件输出为 false）。
	Color bool
}

// OutputOptions 定义自定义输出目标配置。
type OutputOptions struct {
	// Writer 指定输出目标 Writer。
	Writer io.Writer

	// Format 指定输出格式。
	Format OutputFormat

	// Color 指定是否启用颜色输出。
	Color bool

	// TimeFormat 指定时间格式，空值时使用全局默认时间格式。
	TimeFormat string

	// EnableSource 指定是否输出 source 信息，nil 表示使用全局默认设置。
	EnableSource *bool

	// EnableErrorStack 指定是否输出 error 堆栈，nil 表示使用全局默认设置。
	EnableErrorStack *bool
}

// HandlerConfig 定义日志 Handler 的整体配置。
// 该配置负责控制日志级别、格式化、输出目标及轮转行为。
type HandlerConfig struct {
	// Level 指定最低日志等级。
	// 低于该等级的日志将被过滤。
	// 若传入 *LevelVar，可在运行时动态调整日志等级。
	Level slog.Leveler

	// TimeFormat 指定时间格式。
	TimeFormat string

	// CallSkip 指定额外的调用栈跳过层级，用于修正 source 定位。
	CallSkip int

	// EnableSource 指定是否输出 source 信息。
	EnableSource bool

	// EnableErrorStack 指定是否输出 error 堆栈追踪。
	EnableErrorStack bool

	// Outputs 指定自定义输出目标列表。
	Outputs []OutputOptions

	// FileOutputs 指定文件输出目标列表。
	FileOutputs []FileOutputOptions

	// RateLimit 指定日志限流配置。
	// 默认不启用，启用后超出速率的日志将被丢弃。
	RateLimit RateLimitOptions
}

// NewHandlerConfig 使用默认值并应用传入的 Option。
func NewHandlerConfig(options ...Option) *HandlerConfig {
	opts := &HandlerConfig{
		Level:            slog.LevelInfo,
		TimeFormat:       time.RFC3339,
		CallSkip:         0,
		EnableSource:     true,
		EnableErrorStack: false,
	}

	for _, option := range options {
		option(opts)
	}

	if len(opts.Outputs) == 0 && len(opts.FileOutputs) == 0 {
		opts.Outputs = append(opts.Outputs, OutputOptions{
			Writer: os.Stderr,
			Format: OutputText,
			Color:  true,
		})
	}

	return opts
}

// RateLimitOptions 定义日志限流配置。
type RateLimitOptions struct {
	// Enabled 指定是否启用限流。
	Enabled bool

	// RatePerSecond 指定每秒允许输出的日志条数。
	RatePerSecond int

	// Burst 指定允许的突发容量。
	Burst int

	// WarnInterval 指定限流告警的最小输出间隔。
	// 为零时不输出告警日志。
	WarnInterval time.Duration
}

// WithLevel 指定日志等级过滤阈值。
func WithLevel(level slog.Leveler) Option {
	return func(options *HandlerConfig) {
		if level != nil {
			options.Level = level
		}
	}
}

// WithLevelVar 设置可动态调整的日志等级控制器。
func WithLevelVar(level *LevelVar) Option {
	return func(options *HandlerConfig) {
		if level != nil {
			options.Level = level
		}
	}
}

// WithTimeFormat 指定时间格式。
func WithTimeFormat(format string) Option {
	return func(options *HandlerConfig) {
		if format != "" {
			options.TimeFormat = format
		}
	}
}

// WithCallSkip 指定额外调用栈跳过层级。
func WithCallSkip(skip int) Option {
	return func(options *HandlerConfig) {
		if skip >= 0 {
			options.CallSkip = skip
		}
	}
}

// WithSource 指定是否输出 source 信息。
func WithSource(enable bool) Option {
	return func(options *HandlerConfig) {
		options.EnableSource = enable
	}
}

// WithErrorStack 指定是否输出 error 堆栈追踪。
func WithErrorStack(enable bool) Option {
	return func(options *HandlerConfig) {
		options.EnableErrorStack = enable
	}
}

// WithOutput 添加自定义输出目标。
func WithOutput(output OutputOptions) Option {
	return func(options *HandlerConfig) {
		if output.Writer == nil {
			return
		}
		options.Outputs = append(options.Outputs, output)
	}
}

// WithWriterOutput 添加指定 Writer 的输出目标。
func WithWriterOutput(writer io.Writer, format OutputFormat) Option {
	return WithOutput(OutputOptions{
		Writer: writer,
		Format: format,
	})
}

// WithConsoleOutput 添加控制台输出目标。
func WithConsoleOutput(writer io.Writer, format OutputFormat, enableColor bool) Option {
	if writer == nil {
		writer = os.Stderr
	}
	return WithOutput(OutputOptions{
		Writer: writer,
		Format: format,
		Color:  enableColor,
	})
}

// WithFileOutput 添加文件输出目标，并启用轮转及保留策略。
func WithFileOutput(options FileOutputOptions) Option {
	return func(handlerOptions *HandlerConfig) {
		if options.Path == "" {
			return
		}
		handlerOptions.FileOutputs = append(handlerOptions.FileOutputs, options)
	}
}

// WithRateLimit 设置日志限流配置。
func WithRateLimit(options RateLimitOptions) Option {
	return func(handlerOptions *HandlerConfig) {
		handlerOptions.RateLimit = options
	}
}
