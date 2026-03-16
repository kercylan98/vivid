package vivid

import (
	"crypto/tls"
	"time"
)

// ActorSystemRemotingOption 定义一个用来配置 ActorSystemRemotingOptions 的函数签名。
// 开发者可通过一组链式 Option 函数灵活配置远程通信相关的高级参数，实现高度可扩展的定制能力。
type ActorSystemRemotingOption func(options *ActorSystemRemotingOptions)

func NewActorSystemRemotingOptions(opts ...ActorSystemRemotingOption) ActorSystemRemotingOptions {
	options := &ActorSystemRemotingOptions{
		ReconnectLimit:        10,
		ReconnectInitialDelay: 500 * time.Millisecond,
		ReconnectMaxDelay:     10 * time.Second,
		ReconnectFactor:       2.5,
		ReconnectJitter:       true,
		MaxPendingEnvelops:    1024,
		ReadTimeout:           30 * time.Second,
		HeartbeatInterval:     10 * time.Second, // 默认启用心跳，避免空闲连接读超时
		StopTimeout:           10 * time.Minute,
	}
	for _, opt := range opts {
		opt(options)
	}
	return *options
}

// ActorSystemRemotingOptions 封装了 ActorSystem 远程通信组件在运行时的选项参数。
// 新增远程相关的可扩展参数时，建议集中在本结构体内按需扩展，以实现更好的向前兼容和配置集中管理。
type ActorSystemRemotingOptions struct {
	// ReconnectLimit 用于配置远程连接的重试次数。小于 1 则不进行重试。
	ReconnectLimit int

	// ReconnectInitialDelay 用于配置远程连接的重试初始延迟时间。
	ReconnectInitialDelay time.Duration

	// ReconnectMaxDelay 用于配置远程连接的重试最大延迟时间。
	ReconnectMaxDelay time.Duration

	// ReconnectFactor 用于配置远程连接的重试退避因子。
	ReconnectFactor float64

	// ReconnectJitter 用于配置远程连接的重试退避抖动。
	ReconnectJitter bool

	// MaxPendingEnvelops 单个远程地址的最大待发送信封数；超过后新消息将直接失败。
	MaxPendingEnvelops int

	// ReadTimeout 读超时时长，用于 SetReadDeadline；每次成功读完整帧后刷新。
	ReadTimeout time.Duration

	// HeartbeatInterval 心跳发送间隔；为 0 时不发送心跳。默认 10s，保证空闲连接在 ReadTimeout 内能收到心跳。
	HeartbeatInterval time.Duration

	// TLSConfig 可选；非空时 Remoting 服务端使用 TLS 监听，跨 DC/公网部署时建议启用以保证传输加密与身份校验（如 mTLS）。
	TLSConfig *tls.Config

	// StopTimeout 停止超时时间，用于配置远程通信组件的停止超时时间。
	StopTimeout time.Duration
}

// WithActorSystemRemotingOptions 返回一个 ActorSystemOption，用于批量配置 ActorSystem 远程通信的高级选项。
//
// 用法说明：
//   - 首个参数为一个 ActorSystemRemotingOptions 结构体，用于初始化远程选项的默认值；
//   - 其余可变参数为 ActorSystemRemotingOption 函数，可链式定制具体配置；
//   - 推荐通过该方法集中配置包括远程异常、错误处理、重连、连接池等扩展能力。
//
// 典型用法：
//
//	WithActorSystemRemotingOptions(
//	    ActorSystemRemotingOptions{
//	        ConnectionReadFailedHandler: myHandler,
//	    },
//	    func(opt *ActorSystemRemotingOptions) { /* 其它自定义扩展 */ },
//	)
//
// 参数：
//   - options:            远程通信选项的初始配置。
//   - opts ...ActorSystemRemotingOption: 可选参数，链式扩展远程通信选项。
//
// 返回值：
//   - ActorSystemOption:  可传给 NewActorSystem 或其它配置参数的 Option 函数。
func WithActorSystemRemotingOptions(options ActorSystemRemotingOptions, opts ...ActorSystemRemotingOption) ActorSystemOption {
	return func(o *ActorSystemOptions) {
		o.RemotingOptions = &options
		for _, opt := range opts {
			opt(o.RemotingOptions)
		}
	}
}

// WithActorSystemRemotingOption 返回一个 ActorSystemOption，用于批量配置 ActorSystem 远程通信相关选项。
//
// 用法说明：
//   - 支持传入多个 ActorSystemRemotingOption，实现远程相关配置的链式定制。
//   - 该方法不会覆盖已设置的 RemotingOptions 结构体，仅对现有字段进行增量更新。
//
// 参数：
//   - opts ...ActorSystemRemotingOption: 远程通信配置项。
//
// 返回值：
//   - ActorSystemOption: 可传给 NewActorSystem 或其它配置参数的 Option 函数。
func WithActorSystemRemotingOption(opts ...ActorSystemRemotingOption) ActorSystemOption {
	return func(o *ActorSystemOptions) {
		for _, opt := range opts {
			opt(o.RemotingOptions)
		}
	}
}

// WithActorSystemRemotingReconnectLimit 返回一个 ActorSystemRemotingOption，用于配置远程连接的重试次数。
//
// 参数：
//   - limit: 重试次数；小于 1 则不进行重试。
func WithActorSystemRemotingReconnectLimit(limit int) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		if limit >= 0 {
			opts.ReconnectLimit = limit
		}
	}
}

// WithActorSystemRemotingReconnectInitialDelay 返回一个 ActorSystemRemotingOption，用于配置远程连接的重试初始延迟时间。
//
// 参数：
//   - delay: 初始延迟时间；大于 0 时生效。
func WithActorSystemRemotingReconnectInitialDelay(delay time.Duration) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		if delay > 0 {
			opts.ReconnectInitialDelay = delay
		}
	}
}

// WithActorSystemRemotingReconnectMaxDelay 返回一个 ActorSystemRemotingOption，用于配置远程连接的重试最大延迟时间。
//
// 参数：
//   - delay: 最大延迟时间；大于 0 时生效。
func WithActorSystemRemotingReconnectMaxDelay(delay time.Duration) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		if delay > 0 {
			opts.ReconnectMaxDelay = delay
		}
	}
}

// WithActorSystemRemotingReconnectFactor 返回一个 ActorSystemRemotingOption，用于配置远程连接的重试退避因子。
//
// 参数：
//   - factor: 退避因子；大于 0 时生效（通常取 2.0 等值实现指数退避）。
func WithActorSystemRemotingReconnectFactor(factor float64) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		if factor > 0 {
			opts.ReconnectFactor = factor
		}
	}
}

// WithActorSystemRemotingReconnectJitter 返回一个 ActorSystemRemotingOption，用于配置远程连接的重试退避是否启用抖动。
//
// 参数：
//   - jitter: 是否启用抖动，用于缓解重连雷群效应。
func WithActorSystemRemotingReconnectJitter(jitter bool) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		opts.ReconnectJitter = jitter
	}
}

// WithActorSystemRemotingReconnect 返回一个 ActorSystemRemotingOption，用于批量配置远程连接的重试策略。
//
// 参数：
//   - limit: 重试次数；小于 1 则不进行重试。
//   - initialDelay: 初始延迟时间。
//   - maxDelay: 最大延迟时间。
//   - factor: 退避因子，通常为 2.0。
//   - jitter: 是否启用抖动。
func WithActorSystemRemotingReconnect(limit int, initialDelay, maxDelay time.Duration, factor float64, jitter bool) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		if limit >= 0 {
			opts.ReconnectLimit = limit
		}
		if initialDelay > 0 {
			opts.ReconnectInitialDelay = initialDelay
		}
		if maxDelay > 0 {
			opts.ReconnectMaxDelay = maxDelay
		}
		if factor > 0 {
			opts.ReconnectFactor = factor
		}
		opts.ReconnectJitter = jitter
	}
}

// WithActorSystemRemotingMaxPendingEnvelops 返回一个 ActorSystemRemotingOption，用于配置单远程地址的最大待发送信封数。
func WithActorSystemRemotingMaxPendingEnvelops(limit int) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		if limit > 0 {
			opts.MaxPendingEnvelops = limit
		}
	}
}

// WithActorSystemRemotingReadTimeout 返回一个 ActorSystemRemotingOption，用于配置读超时时长。
func WithActorSystemRemotingReadTimeout(d time.Duration) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		if d > 0 {
			opts.ReadTimeout = d
		}
	}
}

// WithActorSystemRemotingHeartbeatInterval 返回一个 ActorSystemRemotingOption，用于配置心跳发送间隔；为 0 时不发送。
func WithActorSystemRemotingHeartbeatInterval(interval time.Duration) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		opts.HeartbeatInterval = interval
	}
}

// WithActorSystemRemotingTLSConfig 返回一个 ActorSystemRemotingOption，用于配置 Remoting 服务端 TLS。
// 非空时服务端使用 TLS 监听；跨 DC/公网部署时建议配置以保证传输加密，可选配合 mTLS 做节点身份校验。
func WithActorSystemRemotingTLSConfig(cfg *tls.Config) ActorSystemRemotingOption {
	return func(opts *ActorSystemRemotingOptions) {
		opts.TLSConfig = cfg
	}
}
