package vivid

import (
    "github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

type (
    // OnLaunch 是 Actor 在启动时候会收到的第一条消息，它标志着 Actor 已经可以正常工作
    //
    // 在收到该消息后，应该对 Actor 生命周期中的资源进行初始化。为了避免状态的泄漏，应避免在构造函数中进行资源的初始化。
    //
    // 这是一个系统消息，每个 Actor 在启动后都会自动收到此消息，无需手动发送。
    // 处理此消息是初始化 Actor 状态的最佳时机。
    OnLaunch = actor.OnLaunch
)

// OnKill 是 Actor 在被终止前会收到的最后一条消息，它标志着 Actor 在处理完该消息后将会被终止
//
// 在收到该消息后，应该对 Actor 生命周期中的资源进行释放或持久化。
//
// 这个消息可以通过 ActorContext.Kill 或 ActorSystem.Kill 方法发送给 Actor，
// 也可以通过 ActorContext.PoisonKill 或 ActorSystem.PoisonKill 方法发送，实现优雅终止。
type OnKill struct {
    m *actor.OnKill // 内部 OnKill 消息实例
}

// Operator 函数将返回对该 Actor 发起终止请求的 Actor 的引用。
func (o *OnKill) Operator() ActorRef {
    return o.m.Operator
}

// Poison 函数将返回该 Actor 是否是优雅终止的，
// 如果是通过 PoisonKill 方法发送的终止请求，则返回 true，表示这是一个优雅终止请求，
// 优雅终止会等待 Actor 处理完当前所有消息后才会被处理。
func (o *OnKill) Poison() bool {
    return o.m.Poison
}

// Reason 函数将返回该 Actor 被终止的原因，
// 这个原因是在调用 Kill 或 PoisonKill 方法时提供的。
func (o *OnKill) Reason() string {
    return o.m.Reason
}

// OnDead 是当子 Actor 生命周期结束后会投递给父 Actor 的消息。它标志着子 Actor 已经被终止，
// 只有通过 Watch 方法监视的 Actor 终止时，才会收到此消息，
// 这个消息可以用于实现监督策略，例如在子 Actor 终止后重启它。
type OnDead struct {
    m *actor.OnDead // 内部 OnDead 消息实例
}

// Ref 函数将返回生命周期结束的子 Actor 的引用。
func (o *OnDead) Ref() ActorRef {
    return o.m.Ref
}

// Pong 是 Ping 操作的响应结构体，包含了往返时间信息，
// 当使用 ActorContext.Ping 或 ActorSystem.Ping 方法时，如果目标 Actor 可达，将返回此结构体，
// 可以通过此结构体获取网络延迟等信息。
type Pong = actor.OnPong
