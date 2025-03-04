package vivid

import (
	"github.com/kercylan98/vivid/src/internal/dedicated"
	"time"
)

const (
	// UserMessage 表示用户消息，该类型消息优先级将低于 SystemMessage
	UserMessage MessageType = iota
	// SystemMessage 表示系统消息，该类型消息优先级为最高
	SystemMessage
)

var (
	_defaultRemoteMessageBuilder RemoteMessageBuilder = &defaultRemoteMessageBuilder{}
)

func init() {
	registerInternalMessage(new(Envelope))
	registerInternalMessage(new(defaultID))
	registerInternalMessage(new(onKill))
	registerInternalMessage(new(onKilled))
	registerInternalMessage(new(onWatch))
	registerInternalMessage(new(onWatchStopped))
	registerInternalMessage(new(onUnwatch))
	registerInternalMessage(new(onPing))
	registerInternalMessage(new(pong))
	registerInternalMessage(new(onLaunch))
	registerInternalMessage(new(onKillFailed))
}

// MessageType 是消息的类型，它用于区分消息的优先级及执行方式
type MessageType = int8

func getDefaultRemoteMessageBuilder() RemoteMessageBuilder {
	return _defaultRemoteMessageBuilder
}

type RemoteMessageBuilder interface {
	IDBuilder
	OnKillBuilder
	OnKilledBuilder
	OnWatchBuilder
	OnWatchStoppedBuilder
	OnUnwatchBuilder
	OnPingBuilder
	PongBuilder
	OnLaunchBuilder
	OnKillFailedBuilder
}

type defaultRemoteMessageBuilder struct {
	defaultIDBuilder
	defaultOnKillBuilder
	defaultOnKilledBuilder
	defaultOnWatchBuilder
	defaultOnWatchStoppedBuilder
	defaultOnUnwatchBuilder
	defaultOnPingBuilder
	defaultOnPongBuilder
	defaultOnLaunchBuilder
	defaultOnKillFailedBuilder
}

var (
	_ Pong = (*pong)(nil)
)

type (
	// PongBuilder 是用于构建 Pong 消息的接口
	PongBuilder interface {
		// BuildPong 构建一个 Pong 消息
		BuildPong(ping OnPing) Pong
	}

	// Pong 该消息反应了一个 Actor 的延迟情况，当通过 ActorContextTransportInteractive.Ping 发起消息后，将会收到该消息
	//  - 该消息支持在跨网络 ActorSystem 间传递
	Pong interface {
		_Pong(mark dedicated.Mark)

		// GetPing 获取 Ping 的时间
		GetPing() time.Time

		// GetPong 获取 Pong 的时间
		GetPong() time.Time
	}

	// DedicatedPong 是 Pong 的专用标记实现，它可以用来实现自定义的 Pong 消息
	DedicatedPong struct{}
)

var (
	_ OnPing = (*onPing)(nil)
)

type (
	// OnPingBuilder 是用于构建 OnPing 消息的接口
	OnPingBuilder interface {
		// BuildOnPing 构建一个 OnPing 消息
		BuildOnPing() OnPing
	}

	// OnPing 是在 ActorSystem 中内部使用的消息类型，由于该消息被内部定义为系统消息，在用户消息中监听该消息仅会得到主动投递的用户级消息
	//  - 自行依赖该消息实现 Ping-Pong 机制不能完整的代表网络通讯的延迟，仅代表 Actor 的消息处理延迟
	//  - 该消息支持在跨网络 ActorSystem 间传递
	OnPing interface {
		_OnPing(mark dedicated.Mark)

		// GetTime 获取发起 Ping 的时间
		GetTime() time.Time
	}

	// DedicatedOnPing 是 OnPing 的专用标记实现，它可以用来实现自定义的 OnPing 消息
	DedicatedOnPing struct{}
)

var (
	_ OnUnwatch = (*onUnwatch)(nil)
)

type (
	// OnUnwatchBuilder 是用于构建 OnUnwatch 消息的接口
	OnUnwatchBuilder interface {
		// BuildOnUnwatch 构建一个 OnUnwatch 消息
		BuildOnUnwatch() OnUnwatch
	}

	// OnUnwatch 是在 ActorSystem 中内部使用的消息类型，它被用于告知 Actor 的观察者已停止对其的观察
	//  - 该消息支持在跨网络 ActorSystem 间传递
	OnUnwatch interface {
		_OnUnwatch(mark dedicated.Mark)
	}

	// DedicatedOnUnwatch 是 OnUnwatch 的专用标记实现，它可以用来实现自定义的 OnUnwatch 消息
	DedicatedOnUnwatch struct{}
)

var (
	_ OnWatchStopped = (*onWatchStopped)(nil)
)

type (
	// OnWatchStoppedBuilder 是用于构建 OnWatchStopped 消息的接口
	OnWatchStoppedBuilder interface {
		// BuildOnWatchStopped 构建一个 OnWatchStopped 消息
		BuildOnWatchStopped(ref ActorRef) OnWatchStopped
	}

	// OnWatchStopped 是用于告知 Actor 所观察的目标 Actor 已经停止运行的消息，该消息将会在目标 Actor 终止时投递给观察者
	//  - 在使用过程中主动投递该消息不会影响内部的观察逻辑，所以在对已观察的目标投递该消息后，当目标 Actor 终止时，将会再次收到该消息，这将可能导致重复处理
	//  - 该消息支持在跨网络 ActorSystem 间传递
	OnWatchStopped interface {
		_OnWatchStopped(mark dedicated.Mark)

		// GetRef 获取已停止观察的 ActorRef
		GetRef() ActorRef
	}

	// DedicatedOnWatchStopped 是 OnWatchStopped 的专用标记实现，它可以用来实现自定义的 OnWatchStopped 消息
	DedicatedOnWatchStopped struct {
		Ref ActorRef
	}
)

var (
	_ OnWatch = (*onWatch)(nil)
)

type (
	// OnWatchBuilder 是用于构建 OnWatch 消息的接口
	OnWatchBuilder interface {
		// BuildOnWatch 构建一个 OnWatch 消息
		BuildOnWatch() OnWatch
	}

	// OnWatch 是在 ActorSystem 中内部使用的消息类型，它被用于告知 Actor 的观察者已开始对其进行观察
	//  - 该消息由于是系统消息，因此在用户消息中监听该消息将会得到主动投递的用户级消息
	//  - 该消息支持在跨网络 ActorSystem 间传递
	OnWatch interface {
		_OnWatch(mark dedicated.Mark)
	}

	// DedicatedOnWatch 是 OnWatch 的专用标记实现，它可以用来实现自定义的 OnWatch 消息
	DedicatedOnWatch struct{}
)

type (
	// OnLaunchBuilder 是用于构建 OnLaunch 消息的接口
	OnLaunchBuilder interface {
		// BuildOnLaunch 构建一个 OnLaunch 消息
		BuildOnLaunch(launchAt time.Time, context map[any]any, isRestart bool) OnLaunch
	}

	// OnLaunch 是在 Actor 启动时的消息，它包含了 Actor 的启动时间、启动上下文以及是否为重启的状态标识
	//  - 该消息在 Actor 启动或重启时被投递，用于初始化 Actor 的状态
	//  - 通常并不建议用户主动投递该消息，如果控制不良将会出现重复初始化等情况
	//  - 该消息支持在跨网络 ActorSystem 间传递
	OnLaunch interface {
		_OnLaunch(mark dedicated.Mark)

		// GetLaunchTime 获取 Actor 启动时间，该时间为 Actor 创建完毕后的时间，而非 Actor 启动时的时间
		GetLaunchTime() time.Time

		// GetContext 获取 Actor 启动上下文中定义的内容，如果不存在则返回 nil 及 false
		//
		// 在一些时候，你也许希望 ActorProvider 返回的是一个单一的 Actor 实例，但在不同的 Actor 启动时，需要传递不同的参数。
		// 通过 GetContext 方法，你可以获取 Actor 启动时传递的参数，以便在 Actor 启动时进行初始化。
		GetContext(key any) (val any, exist bool)

		// Restarted 是否为重启
		Restarted() bool
	}

	DedicatedOnLaunch struct{}
)

var (
	_ OnKill = (*onKill)(nil)
)

type (
	// OnKillBuilder 是用于构建 OnKill 消息的接口
	OnKillBuilder interface {
		// BuildOnKill 构建一个 OnKill 消息
		BuildOnKill(reason string, operator ActorRef, poison bool, restart bool) OnKill
	}

	// OnKill 该消息表示 Actor 在处理完成当前消息后，将会被立即终止。需要在该阶段完成状态的持久化及资源的释放等操作。
	//  - 在 Actor 重启时不会收到该消息
	//  - 该消息支持在跨网络 ActorSystem 间传递
	OnKill interface {
		_OnKill(mark dedicated.Mark)

		// GetReason 获取终止原因
		GetReason() string

		// GetOperator 获取操作者
		GetOperator() ActorRef

		// IsPoison 是否为优雅终止
		IsPoison() bool

		// Restart 是否需要重启
		Restart() bool
	}

	// DedicatedOnKill 是 OnKill 的专用标记实现，它可以用来实现自定义的 OnKill 消息
	DedicatedOnKill struct{}
)

type (
	// OnKillFailedBuilder 是用于构建 OnKillFailed 消息的接口
	OnKillFailedBuilder interface {
		// BuildOnKillFailed 构建一个 OnKillFailed 消息
		BuildOnKillFailed(stack []byte, reason Message, sender ActorRef, message OnKill) OnKillFailed
	}

	// OnKillFailed 在处理 OnKill 消息发生异常时，将会收到该消息，可用于处理异常情况
	//  - 在处理该消息发生异常时，将不会再进行额外的处理，因此需要确保该消息的处理逻辑不会再次发生异常
	//  - 该消息支持在跨网络 ActorSystem 间传递
	//
	// 异常：panic
	OnKillFailed interface {
		_OnKillFailed(mark dedicated.Mark)

		// GetStack 获取异常堆栈
		GetStack() []byte

		// GetReason 获取异常原因
		GetReason() Message

		// GetSender 获取 OnKill 消息发送者
		GetSender() ActorRef

		// GetMessage 获取 OnKill 消息
		GetMessage() OnKill
	}

	// DedicatedOnKillFailed 是 OnKillFailed 的专用标记实现，它可以用来实现自定义的 OnKillFailed 消息
	DedicatedOnKillFailed struct{}
)

var (
	_ OnKilled = (*onKilled)(nil)
)

type (
	OnKilledBuilder interface {
		BuildOnKilled(ref ActorRef) OnKilled
	}

	// OnKilled 是在 ActorSystem 内部使用的消息类型，它被用于告知 Actor 其 Sender（子） 已经终止
	//  - 该消息在 Actor 的 Sender 终止时被投递，用于处理 Actor 的状态
	//  - 该消息支持在跨网络 ActorSystem 间传递
	OnKilled interface {
		_OnKilled(mark dedicated.Mark)
	}

	DedicatedOnKilled struct{}
)

func newEnvelope() *Envelope {
	return &Envelope{}
}

func newStandardEnvelope(senderID, receiverID ID, messageType MessageType, message Message) *Envelope {
	return &Envelope{
		Sender:      senderID,
		Receiver:    receiverID,
		Message:     message,
		MessageType: messageType,
	}
}

func newAgentEnvelope(agent, senderID, receiverID ID, messageType MessageType, message Message) *Envelope {
	return &Envelope{
		Agent:       agent,
		Sender:      senderID,
		Receiver:    receiverID,
		Message:     message,
		MessageType: messageType,
	}
}

// Envelope 是进程间通信的消息包装，包含原始消息内容和附加的头部信息。
type Envelope struct {
	// 消息的类型
	MessageType MessageType

	// 消息代理的 ID
	Agent ID

	// 消息发送者的 ID
	Sender ID

	// 消息接收者的 ID
	Receiver ID

	// 消息的内容
	Message Message
}

type defaultOnKilledBuilder struct{}

func (b *defaultOnKilledBuilder) BuildOnKilled(ref ActorRef) OnKilled {
	return &onKilled{
		Ref: ref,
	}
}

type onKilled struct {
	DedicatedOnKilled
	Ref ActorRef // 已终止 Actor 的 ActorRef
}

func (*DedicatedOnKilled) _OnKilled(mark dedicated.Mark) {}

type defaultOnKillFailedBuilder struct{}

func (b *defaultOnKillFailedBuilder) BuildOnKillFailed(stack []byte, reason Message, sender ActorRef, message OnKill) OnKillFailed {
	return &onKillFailed{
		Stack:   stack,
		Reason:  reason,
		Sender:  sender,
		Message: message,
	}
}

type onKillFailed struct {
	DedicatedOnKillFailed
	Stack   []byte
	Reason  Message
	Sender  ActorRef
	Message OnKill
}

func (*DedicatedOnKillFailed) _OnKillFailed(mark dedicated.Mark) {}

func (o *onKillFailed) GetStack() []byte {
	return o.Stack
}

func (o *onKillFailed) GetReason() Message {
	return o.Reason
}

func (o *onKillFailed) GetSender() ActorRef {
	return o.Sender
}

func (o *onKillFailed) GetMessage() OnKill {
	return o.Message
}

type defaultOnLaunchBuilder struct{}

func (b *defaultOnLaunchBuilder) BuildOnLaunch(launchAt time.Time, context map[any]any, isRestart bool) OnLaunch {
	return &onLaunch{
		LaunchAt:  launchAt,
		Context:   context,
		IsRestart: isRestart,
	}
}

type onLaunch struct {
	DedicatedOnLaunch
	LaunchAt  time.Time
	Context   map[any]any
	IsRestart bool
}

func (o *DedicatedOnLaunch) _OnLaunch(mark dedicated.Mark) {}

func (o *onLaunch) GetLaunchTime() time.Time {
	return o.LaunchAt
}

func (o *onLaunch) GetContext(key any) (val any, exist bool) {
	val, exist = o.Context[key]
	return
}

func (o *onLaunch) Restarted() bool {
	return o.IsRestart
}

type defaultOnKillBuilder struct{}

func (b *defaultOnKillBuilder) BuildOnKill(reason string, operator ActorRef, poison bool, restart bool) OnKill {
	return &onKill{
		Reason:      reason,
		Operator:    operator,
		Poison:      poison,
		NeedRestart: restart,
	}
}

type onKill struct {
	DedicatedOnKill

	Reason      string   // 携带的终止原因
	Operator    ActorRef // 操作者
	Poison      bool     // 是否为优雅终止
	NeedRestart bool     // 是否需要重启
}

func (k *onKill) GetReason() string {
	return k.Reason
}

func (k *onKill) GetOperator() ActorRef {
	return k.Operator
}

func (k *onKill) IsPoison() bool {
	return k.Poison
}

func (k *onKill) Restart() bool {
	return k.NeedRestart
}

func (*DedicatedOnKill) _OnKill(mark dedicated.Mark) {}

type defaultOnWatchBuilder struct{}

func (b *defaultOnWatchBuilder) BuildOnWatch() OnWatch {
	return &onWatch{}
}

type onWatch struct {
	DedicatedOnWatch
}

func (DedicatedOnWatch) _OnWatch(mark dedicated.Mark) {}

var (
	_ OnLaunch = (*onLaunch)(nil)
)

type defaultOnWatchStoppedBuilder struct{}

func (b *defaultOnWatchStoppedBuilder) BuildOnWatchStopped(ref ActorRef) OnWatchStopped {
	return &onWatchStopped{Ref: ref}
}

type onWatchStopped struct {
	DedicatedOnWatchStopped

	Ref ActorRef
}

func (DedicatedOnWatchStopped) _OnWatchStopped(mark dedicated.Mark) {}

func (o *onWatchStopped) GetRef() ActorRef {
	return o.Ref
}

type defaultOnUnwatchBuilder struct{}

func (b *defaultOnUnwatchBuilder) BuildOnUnwatch() OnUnwatch {
	return &onUnwatch{}
}

type onUnwatch struct {
	DedicatedOnUnwatch
}

func (DedicatedOnUnwatch) _OnUnwatch(mark dedicated.Mark) {}

type defaultOnPingBuilder struct{}

func (b *defaultOnPingBuilder) BuildOnPing() OnPing {
	return &onPing{
		Time: time.Now(),
	}
}

type onPing struct {
	DedicatedOnPing
	Time time.Time
}

func (o *onPing) GetTime() time.Time {
	return o.Time
}

func (*DedicatedOnPing) _OnPing(mark dedicated.Mark) {}

type defaultOnPongBuilder struct{}

func (b *defaultOnPongBuilder) BuildPong(ping OnPing) Pong {
	return &pong{
		PingTime:    ping.GetTime(),
		ReceiveTime: time.Now(),
	}
}

type pong struct {
	DedicatedPong
	PingTime    time.Time
	ReceiveTime time.Time
}

func (o *pong) GetPing() time.Time {
	return o.PingTime
}

func (o *pong) GetPong() time.Time {
	return o.ReceiveTime
}

func (*DedicatedPong) _Pong(mark dedicated.Mark) {}
