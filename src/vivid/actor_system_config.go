package vivid

import (
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core/system"
	"github.com/kercylan98/wasteland/src/wasteland"
)

const (
	// GuardDefaultRestartLimit 守护 Actor 在执行监管策略时默认最大重启次数
	GuardDefaultRestartLimit = 10
)

func newActorSystemConfig() *ActorSystemConfig {
	return &ActorSystemConfig{config: &system.Config{
		RPCMessageBuilder:        wasteland.DefaultRPCMessageBuilder(),
		GuardDefaultRestartLimit: GuardDefaultRestartLimit,
	}}
}

type ActorSystemConfig struct {
	config           *system.Config
	globalMonitoring Metrics // 全局监控实例，用于系统级Actor
}

// WithAddress 设置 ActorSystem 的网络地址
func (c *ActorSystemConfig) WithAddress(address string) *ActorSystemConfig {
	c.config.Address = address
	return c
}

// WithLoggerProvider 使用指定的日志提供者
func (c *ActorSystemConfig) WithLoggerProvider(provider log.Provider) *ActorSystemConfig {
	c.config.LoggerProvider = provider
	return c
}

// WithCodec 使用指定的编解码器对跨网络通信进行编解码
func (c *ActorSystemConfig) WithCodec(provider wasteland.CodecProvider, builder wasteland.RPCMessageBuilder) {
	c.config.CodecProvider = provider
	c.config.RPCMessageBuilder = builder
}

// WithGuardDefaultRestartLimit 设置守护 Actor 在执行监管策略时默认最大重启次数
//
// 默认值可参考 GuardDefaultRestartLimit
func (c *ActorSystemConfig) WithGuardDefaultRestartLimit(limit int) *ActorSystemConfig {
	if limit <= 0 {
		panic("invalid guard default restart limit")
	}
	c.config.GuardDefaultRestartLimit = limit
	return c
}

// WithTimingWheelTick 设置定时器滴答时间
//
// 默认值为 100ms
func (c *ActorSystemConfig) WithTimingWheelTick(tick time.Duration) *ActorSystemConfig {
	if tick <= 0 {
		panic("invalid timing wheel tick")
	}
	c.config.TimingWheelTick = tick
	return c
}

// WithTimingWheelSize 设置定时器大小
//
// 默认值为 20
func (c *ActorSystemConfig) WithTimingWheelSize(size int) *ActorSystemConfig {
	if size <= 0 {
		panic("invalid timing wheel size")
	}
	c.config.TimingWheelSize = size
	return c
}

// WithStopTimeout 设置系统停止超时时间
//
// 当设置为0时表示无超时，系统将无限等待停止完成
// 默认值为0（无超时）
func (c *ActorSystemConfig) WithStopTimeout(timeout time.Duration) *ActorSystemConfig {
	if timeout < 0 {
		panic("invalid stop timeout")
	}
	c.config.StopTimeout = timeout
	return c
}

// WithPoisonStopTimeout 设置优雅停止超时时间
//
// 当设置为0时表示无超时，系统将无限等待优雅停止完成
// 默认值为0（无超时）
func (c *ActorSystemConfig) WithPoisonStopTimeout(timeout time.Duration) *ActorSystemConfig {
	if timeout < 0 {
		panic("invalid poison stop timeout")
	}
	c.config.PoisonStopTimeout = timeout
	return c
}

// WithGlobalMonitoring 设置全局监控实例，用于系统级Actor的监控
func (c *ActorSystemConfig) WithGlobalMonitoring(monitoring Metrics) *ActorSystemConfig {
	c.globalMonitoring = monitoring
	return c
}

// WithSimpleMonitoring 配置简单的全局监控（基本功能）
func (c *ActorSystemConfig) WithSimpleMonitoring() *ActorSystemConfig {
	c.globalMonitoring = NewSimpleMetrics()
	return c
}

// WithProductionMonitoring 配置生产环境推荐的全局监控（完整功能）
func (c *ActorSystemConfig) WithProductionMonitoring() *ActorSystemConfig {
	c.globalMonitoring = NewProductionMetrics()
	return c
}

// WithDevelopmentMonitoring 配置开发环境推荐的全局监控（详细调试）
func (c *ActorSystemConfig) WithDevelopmentMonitoring() *ActorSystemConfig {
	c.globalMonitoring = NewDevelopmentMetrics()
	return c
}
