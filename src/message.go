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
	registerInternalMessage(new(envelope))
	registerInternalMessage(new(defaultID))
	registerInternalMessage(new(onKill))
	registerInternalMessage(new(onKilled))
	registerInternalMessage(new(onWatch))
	registerInternalMessage(new(onWatchStopped))
	registerInternalMessage(new(DedicatedOnWatchStopped))
	registerInternalMessage(new(onUnwatch))
	registerInternalMessage(new(onPing))
	registerInternalMessage(new(pong))
}

// MessageType 是消息的类型，它用于区分消息的优先级及执行方式
type MessageType = int8

func getDefaultRemoteMessageBuilder() RemoteMessageBuilder {
	return _defaultRemoteMessageBuilder
}

type RemoteMessageBuilder interface {
	EnvelopeBuilder
	IDBuilder
	OnKillBuilder
	OnKilledBuilder
	OnWatchBuilder
	OnWatchStoppedBuilder
	OnUnwatchBuilder
	OnPingBuilder
	PongBuilder
}

type defaultRemoteMessageBuilder struct {
	defaultEnvelopeBuilder
	defaultIDBuilder
	defaultOnKillBuilder
	defaultOnKilledBuilder
	defaultOnWatchBuilder
	defaultOnWatchStoppedBuilder
	defaultOnUnwatchBuilder
	defaultOnPingBuilder
	defaultOnPongBuilder
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

	// Pong 该消息为 Vivid 内部使用，用于响应 OnPing 消息
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

	// OnPing 该消息为 Vivid 内部使用，用于检测 Actor 是否存活
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

	// OnUnwatch 该消息为 Vivid 内部使用，用于告知 Actor 观察者已不再继续观察
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

	// OnWatchStopped 该消息为 Vivid 内部使用，用于告知 Actor 观察者已停止观察
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

	// OnWatch 该消息为 Vivid 内部使用，用于告知 Actor 被观察
	OnWatch interface {
		_OnWatch(mark dedicated.Mark)
	}

	// DedicatedOnWatch 是 OnWatch 的专用标记实现，它可以用来实现自定义的 OnWatch 消息
	DedicatedOnWatch struct{}
)

type (
	// OnLaunch 在 Actor 启动时，将会作为第一条消息被处理，适用于初始化 Actor 状态等场景。
	OnLaunch interface {
		_OnLaunch(mark dedicated.Mark)

		// GetLaunchTime 获取 Actor 启动时间，该时间为 Actor 创建完毕后的时间，而非 Actor 启动时的时间
		GetLaunchTime() time.Time

		// GetContext 获取 Actor 启动上下文中定义的内容，如果不存在则返回 nil 及 false
		//
		// 在一些时候，你也许希望 ActorProvider 返回的是一个单一的 Actor 实例，但在不同的 Actor 启动时，需要传递不同的参数。
		// 通过 GetContext 方法，你可以获取 Actor 启动时传递的参数，以便在 Actor 启动时进行初始化。
		GetContext(key any) (val any, exist bool)
	}
)

var (
	_ OnKill = (*onKill)(nil)
)

type (
	// OnKillBuilder 是用于构建 OnKill 消息的接口
	OnKillBuilder interface {
		// BuildOnKill 构建一个 OnKill 消息
		BuildOnKill(reason string, operator ActorRef, poison bool) OnKill
	}

	// OnKill 该消息表示 Actor 在处理完成当前消息后，将会被立即终止。需要在该阶段完成状态的持久化及资源的释放等操作。
	OnKill interface {
		_OnKill(mark dedicated.Mark)

		// GetReason 获取终止原因
		GetReason() string

		// GetOperator 获取操作者
		GetOperator() ActorRef

		// IsPoison 是否为优雅终止
		IsPoison() bool
	}

	// DedicatedOnKill 是 OnKill 的专用标记实现，它可以用来实现自定义的 OnKill 消息
	DedicatedOnKill struct{}
)

var (
	_ OnKilled = (*onKilled)(nil)
)

type (
	OnKilledBuilder interface {
		BuildOnKilled(ref ActorRef) OnKilled
	}

	OnKilled interface {
		_OnKilled(mark dedicated.Mark)
	}

	DedicatedOnKilled struct{}
)

var (
	_ Envelope = (*envelope)(nil)
)

type (
	// EnvelopeBuilder 是 Envelope 的构建器，由于 Envelope 支持不同的实现，且包含多种构建方式，因此需要通过构建器来进行创建
	EnvelopeBuilder interface {
		// BuildEnvelope 构建一个空的消息包装，它不包含任何头部信息及消息内容，适用于反序列化场景
		BuildEnvelope() Envelope

		// BuildStandardEnvelope 构建一个标准的消息包装，它包含了消息的发送者、接收者、消息内容及消息类型
		BuildStandardEnvelope(senderID ID, receiverID ID, messageType MessageType, message Message) Envelope

		// BuildAgentEnvelope 构建一个代理的消息包装，它与标准消息包装相似，但是实际发送人为代理 Actor
		BuildAgentEnvelope(agent, senderID, receiverID ID, messageType MessageType, message Message) Envelope
	}

	// Envelope 是进程间通信的消息包装，包含原始消息内容和附加的头部信息，支持跨网络传输。
	//   - 如果需要支持其他序列化方式，可以通过实现 Envelope 接口并自定义消息包装，同时实现 EnvelopeBuilder 接口来提供构建方式。
	Envelope interface {
		// GetAgent 获取消息代理的 ID
		GetAgent() ID

		// GetSender 获取消息发送者的 ID
		GetSender() ID

		// GetReceiver 获取消息接收者的 ID
		GetReceiver() ID

		// GetMessage 获取消息的内容
		GetMessage() Message

		// GetMessageType 获取消息的类型
		GetMessageType() MessageType
	}
)

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

func newOnLaunch(launchAt time.Time, context map[any]any) OnLaunch {
	return &onLaunch{
		launchAt: launchAt,
		context:  context,
	}
}

type onLaunch struct {
	launchAt time.Time
	context  map[any]any
}

func (o *onLaunch) _OnLaunch(mark dedicated.Mark) {}

func (o *onLaunch) GetLaunchTime() time.Time {
	return o.launchAt
}

func (o *onLaunch) GetContext(key any) (val any, exist bool) {
	val, exist = o.context[key]
	return
}

type defaultOnKillBuilder struct{}

func (b *defaultOnKillBuilder) BuildOnKill(reason string, operator ActorRef, poison bool) OnKill {
	return &onKill{
		Reason:   reason,
		Operator: operator,
		Poison:   poison,
	}
}

type onKill struct {
	DedicatedOnKill

	Reason   string   // 携带的终止原因
	Operator ActorRef // 操作者
	Poison   bool     // 是否为优雅终止
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

func (*DedicatedOnKill) _OnKill(mark dedicated.Mark) {}

// defaultEnvelopeBuilder 是 EnvelopeBuilder 的默认实现，它提供了 envelope 的构建方式
type defaultEnvelopeBuilder struct{}

func (d *defaultEnvelopeBuilder) BuildEnvelope() Envelope {
	return &envelope{}
}

func (d *defaultEnvelopeBuilder) BuildStandardEnvelope(senderID, receiverID ID, messageType MessageType, message Message) Envelope {
	return &envelope{
		Sender:      senderID,
		Receiver:    receiverID,
		Message:     message,
		MessageType: messageType,
	}
}

func (d *defaultEnvelopeBuilder) BuildAgentEnvelope(agent, senderID, receiverID ID, messageType MessageType, message Message) Envelope {
	return &envelope{
		Agent:       agent,
		Sender:      senderID,
		Receiver:    receiverID,
		Message:     message,
		MessageType: messageType,
	}
}

// envelope 是 Envelope 的默认实现，它基于 gob 序列化方式实现了 Envelope 接口
type envelope struct {
	Agent       ID
	Sender      ID
	Receiver    ID
	Message     Message
	MessageType MessageType
}

func (d *envelope) GetAgent() ID {
	return d.Agent
}

func (d *envelope) GetSender() ID {
	return d.Sender
}

func (d *envelope) GetReceiver() ID {
	return d.Receiver
}

func (d *envelope) GetMessage() Message {
	return d.Message
}

func (d *envelope) GetMessageType() MessageType {
	return d.MessageType
}

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
