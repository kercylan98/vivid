package vivid

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/internal/utils/options"
	"log/slog"
	"time"
)

var (
	_ ActorConfiguration = (*defaultActorConfig)(nil)
)

// NewActorConfig 创建 Actor 的配置
func NewActorConfig(parent ActorContext) ActorConfiguration {
	var loggerProvider log.Provider
	if parent != nil {
		loggerProvider = parent.getLoggerProvider()
	}
	c := &defaultActorConfig{
		loggerProvider:     loggerProvider,
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
	WithReadOnly() ActorConfiguration

	// WithLoggerProvider 设置 Actor 的日志记录器获取器
	WithLoggerProvider(provider log.Provider) ActorConfiguration

	// WithName 设置 Actor 的名称
	WithName(name string) ActorConfiguration

	// WithDispatcher 设置 Actor 的调度器
	WithDispatcher(provider DispatcherProvider) ActorConfiguration

	// WithMailbox 设置 Actor 的邮箱
	WithMailbox(provider MailboxProvider) ActorConfiguration

	// WithLaunchContextProvider 设置 Actor 的启动上下文提供者
	//  - 通过使用提供者的方式，允许 Actor 在每一次启动时都能获取到不同的启动上下文
	//
	// 提供者如果返回的是空指针，不会引发任何异常，但会导致 Actor 在启动时无法获取到启动上下文
	WithLaunchContextProvider(provider LaunchContextProvider) ActorConfiguration

	// WithSupervisor 设置 Actor 的监管者，监管者用于对 Actor 异常情况进行监管策略的执行
	WithSupervisor(supervisor Supervisor) ActorConfiguration

	// WithTimingWheel 设置 Actor 的定时器
	//  - 如果 Actor 需要使用大量的定时器，可通过该选项指定独立的定时器
	//  - 默认使用的是 ActorSystem 的全局定时器
	WithTimingWheel(timing timing.Wheel) ActorConfiguration

	// WithSlowMessageThreshold 设置 Actor 的慢消息阈值，覆盖 ActorSystem 的全局慢消息阈值
	//  - 用于设置 Actor 处理消息的阈值，当消息处理时间超过该阈值时，会记录一条 WARN 级别日志
	//  - 当阈值为 <= 0 时，不会记录任何日志
	WithSlowMessageThreshold(threshold time.Duration) ActorConfiguration

	// WithPersistent 设置 Actor 为持久化 Actor
	//  - 持久化 Actor 会将持久化消息进行记录，并在 Actor 重启时进行恢复
	//  - 设置 interval 可指定持久化消息的自动存储间隔，当 <= 0 时，不会自动存储，需要主动调用 ActorContextPersistent.Persist 方法
	WithPersistent(persistentId string, interval time.Duration, storage PersistentStorage) ActorConfiguration
}

// ActorOptionsFetcher 是 Actor 的配置获取接口
type ActorOptionsFetcher interface {

	// FetchReadOnly 获取 Actor 的配置是否为只读
	FetchReadOnly() bool

	// FetchLogger 获取 Actor 的日志记录器获取器
	FetchLogger() log.Logger

	// FetchName 获取 Actor 的名称
	FetchName() string

	// FetchDispatcher 获取 Actor 的调度器
	FetchDispatcher() DispatcherProvider

	// FetchMailbox 获取 Actor 的邮箱
	FetchMailbox() MailboxProvider

	// FetchLoggerProvider 获取 Actor 的日志记录器获取器
	FetchLoggerProvider() log.Provider

	// FetchLaunchContextProvider 获取 Actor 的启动上下文提供者
	FetchLaunchContextProvider() LaunchContextProvider

	// FetchSupervisor 获取 Actor 的监管者
	FetchSupervisor() Supervisor

	// FetchTimingWheel 获取 Actor 的定时器
	FetchTimingWheel() timing.Wheel

	// FetchSlowMessageThreshold 获取 Actor 的慢消息阈值
	FetchSlowMessageThreshold() time.Duration

	// FetchPersistentId 获取 Actor 的持久化 ID
	FetchPersistentId() string

	// FetchPersistentInterval 获取 Actor 的持久化间隔
	FetchPersistentInterval() time.Duration

	// FetchPersistentStorage 获取 Actor 的持久化存储器
	FetchPersistentStorage() PersistentStorage
}

type defaultActorConfig struct {
	options.LogicOptions[ActorOptionsFetcher, ActorOptions]
	readOnly              bool                  // 是否只读
	loggerProvider        log.Provider          // 日志记录器提供者
	name                  string                // 名称
	dispatcherProvider    DispatcherProvider    // 调度器
	mailboxProvider       MailboxProvider       // 邮箱
	launchContextProvider LaunchContextProvider // 启动上下文提供者
	supervisor            Supervisor            // 监管者
	timingWheel           timing.Wheel          // 定时器
	slownessThreshold     time.Duration         // 慢消息阈值
	persistentId          string                // 持久化 ID
	persistentInterval    time.Duration         // 持久化间隔
	persistentStorage     PersistentStorage     // 持久化存储器
}

func (d *defaultActorConfig) WithSlowMessageThreshold(threshold time.Duration) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.slownessThreshold = threshold
	}
	return d
}

func (d *defaultActorConfig) FetchSlowMessageThreshold() time.Duration {
	return d.slownessThreshold
}

func (d *defaultActorConfig) WithSupervisor(supervisor Supervisor) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.supervisor = supervisor
	}
	return d
}

func (d *defaultActorConfig) FetchSupervisor() Supervisor {
	return d.supervisor
}

func (d *defaultActorConfig) WithLaunchContextProvider(provider LaunchContextProvider) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.launchContextProvider = provider
	}
	return d
}

func (d *defaultActorConfig) FetchLaunchContextProvider() LaunchContextProvider {
	return d.launchContextProvider
}

func (d *defaultActorConfig) FetchLoggerProvider() log.Provider {
	return d.loggerProvider
}

func (d *defaultActorConfig) WithDispatcher(provider DispatcherProvider) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.dispatcherProvider = provider
	}
	return d
}

func (d *defaultActorConfig) WithMailbox(provider MailboxProvider) ActorConfiguration {
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

func (d *defaultActorConfig) WithName(name string) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.name = name
	}
	return d
}

func (d *defaultActorConfig) FetchName() string {
	return d.name
}

func (d *defaultActorConfig) WithLoggerProvider(provider log.Provider) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.loggerProvider = provider
	}
	return d
}

func (d *defaultActorConfig) FetchLogger() log.Logger {
	if d.loggerProvider == nil {
		logger := defaultLoggerProvider()
		logger.Warn("FetchLogger", slog.String("info", "LoggerFetcher is nil, use default logger"))
		return logger
	}
	logger := d.loggerProvider.Provide()
	if logger == nil {
		logger = defaultLoggerProvider()
		logger.Warn("FetchLogger", slog.String("info", "nil Logger from LoggerFetcher, use default logger"))
		return defaultLoggerProvider()
	}
	return logger
}

func (d *defaultActorConfig) WithReadOnly() ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.readOnly = true
	}

	return d
}

func (d *defaultActorConfig) FetchReadOnly() bool {
	return d.readOnly
}

func (d *defaultActorConfig) WithTimingWheel(timing timing.Wheel) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.timingWheel = timing
	}
	return d
}

func (d *defaultActorConfig) FetchTimingWheel() timing.Wheel {
	return d.timingWheel
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

type LaunchContextProvider interface {
	Provide() map[any]any
}

type LaunchContextProviderFn func() map[any]any

func (f LaunchContextProviderFn) Provide() map[any]any {
	return f()
}

func (d *defaultActorConfig) WithPersistent(persistentId string, interval time.Duration, storage PersistentStorage) ActorConfiguration {
	if !d.modifyReadOnlyCheck() {
		if persistentId != "" && storage != nil {
			d.persistentId = persistentId
			d.persistentInterval = interval
			d.persistentStorage = storage
		}
	}
	return d
}

func (d *defaultActorConfig) FetchPersistentId() string {
	return d.persistentId
}

func (d *defaultActorConfig) FetchPersistentInterval() time.Duration {
	return d.persistentInterval
}

func (d *defaultActorConfig) FetchPersistentStorage() PersistentStorage {
	return d.persistentStorage
}
