package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

type (
	// OnLaunch 是 Actor 在启动时候会收到的第一条消息，它标志着 Actor 已经可以正常工作
	//
	// 在收到该消息后，应该对 Actor 生命周期中的资源进行初始化。为了避免状态的泄漏，应避免在构造函数中进行资源的初始化。
	OnLaunch = actor.OnLaunch
)

// OnKill 是 Actor 在被终止前会收到的最后一条消息，它标志着 Actor 在处理完该消息后将会被终止
//
// 在收到该消息后，应该对 Actor 生命周期中的资源进行释放或持久化
type OnKill struct {
	m *actor.OnKill
}

// Operator 函数将返回对该 Actor 发起终止请求的 Actor 的引用
func (o *OnKill) Operator() ActorRef {
	return o.m.Operator
}

// Poison 函数将返回该 Actor 是否是优雅终止的
func (o *OnKill) Poison() bool {
	return o.m.Poison
}

// Reason 函数将返回该 Actor 被终止的原因
func (o *OnKill) Reason() string {
	return o.m.Reason
}

// OnDead 是当子 Actor 生命周期结束后会投递给父 Actor 的消息。它标志着子 Actor 已经被终止
type OnDead struct {
	m *actor.OnDead
}

// Ref 函数将返回生命周期结束的子 Actor 的引用
func (o *OnDead) Ref() ActorRef {
	return o.m.Ref
}
