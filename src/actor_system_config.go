package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/internal/utils/hash"
	"github.com/kercylan98/vivid/src/internal/utils/options"
	"log/slog"
	"net"
	"time"
)

var (
	_ ActorSystemConfiguration = (*defaultActorSystemConfig)(nil)
)

// NewActorSystemConfig 创建一个用于 ActorSystem 的默认配置器
func NewActorSystemConfig() ActorSystemConfiguration {
	c := &defaultActorSystemConfig{
		name: hash.GenerateHash(),
		loggerProvider: log.ProviderFn(func() log.Logger {
			return log.GetDefault()
		}),
		remoteMessageBuilder: getDefaultRemoteMessageBuilder(),
		codec: CodecProviderFn(func() Codec {
			return newGobCodec()
		}),
		defaultSupervisorRestartLimit: 10,
		slowMessageThreshold:          500 * time.Millisecond,
	}
	c.LogicOptions = options.NewLogicOptions[ActorSystemOptionsFetcher, ActorSystemOptions](c, c)
	return c
}

// ActorSystemConfigurator 是 ActorSystem 的配置接口，它允许结构化的配置 ActorSystem
type ActorSystemConfigurator interface {
	// Configure 配置 ActorSystem
	Configure(config ActorSystemConfiguration)
}

// ActorSystemConfiguratorFn 是 ActorSystem 的配置接口，它允许通过函数式的方式配置 ActorSystem
type ActorSystemConfiguratorFn func(config ActorSystemConfiguration)

func (f ActorSystemConfiguratorFn) Configure(config ActorSystemConfiguration) {
	f(config)
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
	WithReadOnly() ActorSystemOptionsFetcher

	// WithName 设置 ActorSystem 的名称
	WithName(name string) ActorSystemConfiguration

	// WithLoggerProvider 设置 ActorSystem 的日志记录器提供者
	//   - 提供者不应在每次都返回一个新的示例，应返回当前所使用的示例
	WithLoggerProvider(provider log.Provider) ActorSystemConfiguration

	// WithCodec 设置 ActorSystem 的编解码器，仅在通过 WithListen 设置了监听地址后有效
	WithCodec(codec CodecProvider, builder RemoteMessageBuilder) ActorSystemConfiguration

	// WithListen 设置 ActorSystem 的监听地址，该方法会覆盖 WithListener 方法
	//  - 监听地址应该为一个有效的 TCP 地址，例如 ":8080"
	//  - 当监听地址不合法时，将会产生 panic
	WithListen(address string) ActorSystemOptionsFetcher

	// WithListener 设置 ActorSystem 的监听器，该方法会覆盖 WithListen 方法
	WithListener(listener net.Listener) ActorSystemOptionsFetcher

	// WithDefaultSupervisorRestartLimit 设置 ActorSystem 的默认 Supervisor 重启次数限制
	WithDefaultSupervisorRestartLimit(limit int) ActorSystemOptionsFetcher

	// WithSlowMessageThreshold 设置 ActorSystem 的慢消息阈值
	//  - 当消息处理时间超过阈值时，将会产生一条警告日志
	//  - 阈值为 0 时，表示关闭慢消息检测，默认值为 500ms
	WithSlowMessageThreshold(threshold time.Duration) ActorSystemOptionsFetcher
}

// ActorSystemOptionsFetcher 是 ActorSystem 的配置选项获取器
type ActorSystemOptionsFetcher interface {
	// FetchReadOnly 获取 ActorSystem 的配置是否为只读
	FetchReadOnly() bool

	// FetchName 获取 ActorSystem 的名称
	FetchName() string

	// FetchLogger 获取 ActorSystem 的日志记录器
	FetchLogger() log.Logger

	// FetchLoggerProvider 获取 ActorSystem 的日志记录器提供者
	FetchLoggerProvider() log.Provider

	// FetchCodec 获取 ActorSystem 的编解码器
	FetchCodec() CodecProvider

	// FetchRemoteMessageBuilder 获取 ActorSystem 的远程消息构建器
	FetchRemoteMessageBuilder() RemoteMessageBuilder

	// FetchListener 获取 ActorSystem 的监听器
	FetchListener() net.Listener

	// FetchDefaultSupervisorRestartLimit 获取 ActorSystem 的默认 Supervisor 重启次数限制
	FetchDefaultSupervisorRestartLimit() int

	// FetchSlowMessageThreshold 获取 ActorSystem 的慢消息阈值
	FetchSlowMessageThreshold() time.Duration
}

type defaultActorSystemConfig struct {
	options.LogicOptions[ActorSystemOptionsFetcher, ActorSystemOptions]
	readOnly                      bool                 // 是否只读
	name                          string               // ActorSystem 的名称
	loggerProvider                log.Provider         // 日志记录器获取器
	codec                         CodecProvider        // 编解码器
	remoteMessageBuilder          RemoteMessageBuilder // 远程消息构建器
	listener                      net.Listener         // 监听器
	defaultSupervisorRestartLimit int                  // 默认 Supervisor 重启次数限制
	slowMessageThreshold          time.Duration        // 慢消息阈值
}

func (d *defaultActorSystemConfig) WithSlowMessageThreshold(threshold time.Duration) ActorSystemOptionsFetcher {
	if !d.modifyReadOnlyCheck() {
		d.slowMessageThreshold = threshold
	}
	return d
}

func (d *defaultActorSystemConfig) FetchSlowMessageThreshold() time.Duration {
	return d.slowMessageThreshold
}

func (d *defaultActorSystemConfig) WithCodec(codec CodecProvider, builder RemoteMessageBuilder) ActorSystemConfiguration {
	d.codec = codec
	d.remoteMessageBuilder = builder
	return d
}

func (d *defaultActorSystemConfig) FetchCodec() CodecProvider {
	return d.codec
}

func (d *defaultActorSystemConfig) FetchRemoteMessageBuilder() RemoteMessageBuilder {
	return d.remoteMessageBuilder
}

func (d *defaultActorSystemConfig) FetchLoggerProvider() log.Provider {
	return d.loggerProvider
}

func (d *defaultActorSystemConfig) WithLoggerProvider(provider log.Provider) ActorSystemConfiguration {
	if !d.modifyReadOnlyCheck() {
		d.loggerProvider = provider
	}
	return d
}

func (d *defaultActorSystemConfig) FetchLogger() log.Logger {
	if d.loggerProvider == nil {
		logger := defaultLoggerProvider()
		logger.Warn("FetchLogger", slog.String("info", "nil LoggerProvider, use default logger"))
		return logger
	}
	logger := d.loggerProvider.Provide()
	if logger == nil {
		logger = defaultLoggerProvider()
		logger.Warn("FetchLogger", slog.String("info", "nil Logger from LoggerFetcher, use default logger"))
		return logger
	}
	return logger
}

func (d *defaultActorSystemConfig) InitDefault() ActorSystemConfiguration {
	d.readOnly = true
	return d
}

func (d *defaultActorSystemConfig) WithReadOnly() ActorSystemOptionsFetcher {
	if !d.modifyReadOnlyCheck() {
		d.readOnly = true
	}
	return d
}

func (d *defaultActorSystemConfig) FetchReadOnly() bool {
	return d.readOnly
}

func (d *defaultActorSystemConfig) WithName(name string) ActorSystemConfiguration {
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

func (d *defaultActorSystemConfig) WithListen(address string) ActorSystemOptionsFetcher {
	if !d.modifyReadOnlyCheck() {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			panic(err)
		}
		return d.WithListener(listener)
	}
	return d
}

func (d *defaultActorSystemConfig) WithListener(listener net.Listener) ActorSystemOptionsFetcher {
	if !d.modifyReadOnlyCheck() {
		d.listener = listener
	}
	return d
}

func (d *defaultActorSystemConfig) FetchListener() net.Listener {
	return d.listener
}

func (d *defaultActorSystemConfig) WithDefaultSupervisorRestartLimit(limit int) ActorSystemOptionsFetcher {
	if !d.modifyReadOnlyCheck() {
		d.defaultSupervisorRestartLimit = limit
	}
	return d
}

func (d *defaultActorSystemConfig) FetchDefaultSupervisorRestartLimit() int {
	return d.defaultSupervisorRestartLimit
}
