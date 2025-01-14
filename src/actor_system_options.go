package vivid

import (
	"github.com/kercylan98/vivid/src/internal/utils/hash"
	"github.com/kercylan98/vivid/src/internal/utils/options"
	"log/slog"
)

var (
	_ ActorSystemConfiguration = (*defaultActorSystemConfig)(nil)
)

// NewActorSystemConfig 创建一个用于 ActorSystem 的默认配置器
func NewActorSystemConfig() ActorSystemConfiguration {
	c := &defaultActorSystemConfig{
		name:          hash.GenerateHash(),
		loggerFetcher: LoggerFetcherFn(defaultLoggerFetcher),
	}
	c.LogicOptions = options.NewLogicOptions[ActorSystemOptionsFetcher, ActorSystemOptions](c, c)
	return c
}

// ActorSystemConfigurator 是 ActorSystem 的配置接口，它允许结构化的配置 ActorSystem
type ActorSystemConfigurator interface {
	// Configure 配置 ActorSystem
	Configure(config ActorSystemConfiguration) ActorSystem
}

// ActorSystemConfiguratorFn 是 ActorSystem 的配置接口，它允许通过函数式的方式配置 ActorSystem
type ActorSystemConfiguratorFn func(config ActorSystemConfiguration) ActorSystem

func (f ActorSystemConfiguratorFn) Configure(config ActorSystemConfiguration) ActorSystem {
	return f(config)
}

// ActorSystemConfiguration 是 ActorSystem 的配置接口
type ActorSystemConfiguration interface {
	ActorSystemOptions
	ActorSystemOptionsFetcher

	// InitDefault 初始化 ActorSystem 的默认配置
	InitDefault() ActorSystemConfiguration
}

// ActorSystemOptions 是 ActorSystem 的配置选项
type ActorSystemOptions interface {
	options.LogicOptions[ActorSystemOptionsFetcher, ActorSystemOptions]

	// WithReadOnly 设置 ActorSystem 的配置为只读
	WithReadOnly() ActorSystemOptions

	// WithName 设置 ActorSystem 的名称
	WithName(name string) ActorSystemOptions

	// WithLoggerFetcher 设置 ActorSystem 的日志记录器获取器
	WithLoggerFetcher(fetcher LoggerFetcher) ActorSystemOptions
}

// ActorSystemOptionsFetcher 是 ActorSystem 的配置选项获取器
type ActorSystemOptionsFetcher interface {
	// FetchReadOnly 获取 ActorSystem 的配置是否为只读
	FetchReadOnly() bool

	// FetchName 获取 ActorSystem 的名称
	FetchName() string

	// FetchLoggerFetcher 获取 ActorSystem 的日志记录器获取器
	FetchLoggerFetcher() LoggerFetcher

	// FetchLogger 获取 ActorSystem 的日志记录器
	FetchLogger() *Logger
}

type defaultActorSystemConfig struct {
	options.LogicOptions[ActorSystemOptionsFetcher, ActorSystemOptions]
	readOnly      bool          // 是否只读
	name          string        // ActorSystem 的名称
	loggerFetcher LoggerFetcher // 日志记录器获取器
}

func (d *defaultActorSystemConfig) WithLoggerFetcher(fetcher LoggerFetcher) ActorSystemOptions {
	if !d.modifyReadOnlyCheck() {
		d.loggerFetcher = fetcher
	}
	return d
}

func (d *defaultActorSystemConfig) FetchLoggerFetcher() LoggerFetcher {
	return d.loggerFetcher
}

func (d *defaultActorSystemConfig) FetchLogger() *Logger {
	if d.loggerFetcher == nil {
		logger := defaultLoggerFetcher()
		logger.Warn("FetchLogger", slog.String("info", "LoggerFetcher is nil, use default logger"))
		return logger
	}
	logger := d.loggerFetcher.Fetch()
	if logger == nil {
		logger = defaultLoggerFetcher()
		logger.Warn("FetchLogger", slog.String("info", "nil Logger from LoggerFetcher, use default logger"))
		return defaultLoggerFetcher()
	}
	return logger
}

func (d *defaultActorSystemConfig) InitDefault() ActorSystemConfiguration {
	d.readOnly = true
	return d
}

func (d *defaultActorSystemConfig) WithReadOnly() ActorSystemOptions {
	if !d.modifyReadOnlyCheck() {
		d.readOnly = true
	}
	return d
}

func (d *defaultActorSystemConfig) FetchReadOnly() bool {
	return d.readOnly
}

func (d *defaultActorSystemConfig) WithName(name string) ActorSystemOptions {
	if !d.modifyReadOnlyCheck() {
		d.name = name
	}
	return d
}

func (d *defaultActorSystemConfig) FetchName() string {
	return d.name
}

func (d *defaultActorSystemConfig) modifyReadOnlyCheck() bool {
	if d.readOnly {
		d.FetchLogger().Warn("ActorSystemOptions", slog.String("info", "options is read-only, modify invalid"))
	}
	return d.readOnly
}
