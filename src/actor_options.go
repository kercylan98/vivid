package vivid

import (
	"github.com/kercylan98/vivid/src/internal/utils/options"
	"log/slog"
)

var (
	_ ActorConfiguration = (*defaultActorConfig)(nil)
)

// NewActorConfig 创建 Actor 的配置
func NewActorConfig(parent ActorContext) ActorConfiguration {
	var loggerFetcher LoggerFetcher
	if parent != nil {
		loggerFetcher = parent.GetLoggerFetcher()
	}
	c := &defaultActorConfig{
		loggerFetcher:      loggerFetcher,
		dispatcherProvider: DispatcherProviderFn(defaultDispatcherProvider),
		mailboxProvider:    MailboxProviderFn(defaultMailboxProvider),
	}
	c.LogicOptions = options.NewLogicOptions[ActorOptionsFetcher, ActorOptions](c, c)
	return c
}

// ActorConfigurator 是 Actor 的配置接口
type ActorConfigurator interface {
	// Configure 配置 Actor
	Configure(config ActorConfiguration)
}

// ActorConfiguratorFn 是 Actor 的配置函数接口
type ActorConfiguratorFn func(config ActorConfiguration)

// Configure 配置 Actor
func (f ActorConfiguratorFn) Configure(config ActorConfiguration) {
	f(config)
}

// ActorConfiguration 是 Actor 的配置接口
type ActorConfiguration interface {
	ActorOptions
	ActorOptionsFetcher

	// InitDefault 初始化 Actor 的默认配置
	InitDefault() ActorConfiguration
}

// ActorOptions 是 Actor 的配置接口，描述了 Actor 的各项行为
type ActorOptions interface {
	options.LogicOptions[ActorOptionsFetcher, ActorOptions]

	// WithReadOnly 设置 Actor 的配置为只读
	WithReadOnly() ActorOptionsFetcher

	// WithLoggerFetcher 设置 Actor 的日志记录器获取器
	WithLoggerFetcher(fetcher LoggerFetcher) ActorOptions

	// WithName 设置 Actor 的名称
	WithName(name string) ActorOptions

	// WithDispatcher 设置 Actor 的调度器
	WithDispatcher(provider DispatcherProvider) ActorOptions

	// WithMailbox 设置 Actor 的邮箱
	WithMailbox(provider MailboxProvider) ActorOptions
}

// ActorOptionsFetcher 是 Actor 的配置获取接口
type ActorOptionsFetcher interface {

	// FetchReadOnly 获取 Actor 的配置是否为只读
	FetchReadOnly() bool

	// FetchLoggerFetcher 获取 Actor 的日志记录器获取器
	FetchLoggerFetcher() LoggerFetcher

	// FetchLogger 获取 Actor 的日志记录器
	FetchLogger() *Logger

	// FetchName 获取 Actor 的名称
	FetchName() string

	// FetchDispatcher 获取 Actor 的调度器
	FetchDispatcher() DispatcherProvider

	// FetchMailbox 获取 Actor 的邮箱
	FetchMailbox() MailboxProvider
}

type defaultActorConfig struct {
	options.LogicOptions[ActorOptionsFetcher, ActorOptions]
	readOnly           bool               // 是否只读
	loggerFetcher      LoggerFetcher      // 日志记录器获取器
	name               string             // 名称
	dispatcherProvider DispatcherProvider // 调度器
	mailboxProvider    MailboxProvider    // 邮箱
}

func (d *defaultActorConfig) WithDispatcher(provider DispatcherProvider) ActorOptions {
	if !d.modifyReadOnlyCheck() {
		d.dispatcherProvider = provider
	}
	return d
}

func (d *defaultActorConfig) WithMailbox(provider MailboxProvider) ActorOptions {
	if !d.modifyReadOnlyCheck() {
		d.mailboxProvider = provider
	}
	return d
}

func (d *defaultActorConfig) FetchDispatcher() DispatcherProvider {
	return d.dispatcherProvider
}

func (d *defaultActorConfig) FetchMailbox() MailboxProvider {
	return d.mailboxProvider
}

func (d *defaultActorConfig) WithName(name string) ActorOptions {
	if !d.modifyReadOnlyCheck() {
		d.name = name
	}
	return d
}

func (d *defaultActorConfig) FetchName() string {
	return d.name
}

func (d *defaultActorConfig) WithLoggerFetcher(fetcher LoggerFetcher) ActorOptions {
	if !d.modifyReadOnlyCheck() {
		d.loggerFetcher = fetcher
	}
	return d
}

func (d *defaultActorConfig) FetchLoggerFetcher() LoggerFetcher {
	return d.loggerFetcher
}

func (d *defaultActorConfig) FetchLogger() *Logger {
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

func (d *defaultActorConfig) WithReadOnly() ActorOptionsFetcher {
	if !d.modifyReadOnlyCheck() {
		d.readOnly = true
	}

	return d
}

func (d *defaultActorConfig) FetchReadOnly() bool {
	return d.readOnly
}

func (d *defaultActorConfig) InitDefault() ActorConfiguration {
	d.readOnly = true
	return d
}

func (d *defaultActorConfig) modifyReadOnlyCheck() bool {
	if d.readOnly {
		d.FetchLogger().Warn("ActorOptions", slog.String("info", "options is read-only, modify invalid"))
	}
	return d.readOnly
}
