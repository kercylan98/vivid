package vivid

import "time"

// VirtualActorProvider 按逻辑身份创建虚拟 Actor；首次激活、钝化后激活及重启均调用此接口。
// 不同身份可以并行初始化，返回值必须是有效的非 nil Actor。
type VirtualActorProvider interface {
	Provide(kind, name string) Actor
}

// VirtualActorProviderFN 以函数实现 VirtualActorProvider。
type VirtualActorProviderFN func(kind, name string) Actor

func (fn VirtualActorProviderFN) Provide(kind, name string) Actor { return fn(kind, name) }

// VirtualActorOptions 配置本系统内的按需激活与空闲钝化。
type VirtualActorOptions struct {
	IdleTimeout           time.Duration // 空闲钝化时间；0 使用 10 分钟，负值关闭自动钝化
	ActivationConcurrency int           // 并行初始化数；0 使用 16
	PendingMessageLimit   int           // 等待激活或钝化结束的消息总上限；0 使用 4096
}

// WithActorSystemVirtualActorOptions 配置虚拟 Actor；负数并发或等待上限在 Start 时返回错误。
func WithActorSystemVirtualActorOptions(options VirtualActorOptions) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.VirtualActorOptions = options
	}
}
