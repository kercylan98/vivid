package vivid

import "github.com/kercylan98/vivid/pkg/provider"

type (
	// Message 定义了 Actor 系统中消息的通用类型。
	//
	// 在 vivid 框架中，任何类型的数据都可以作为消息在 Actor 之间传递。
	Message           = any
	internalMessageId = string
	internalMessage   interface {
		marshal() (internalMessageId, []byte)
		unmarshal(b []byte)
	}
)

const (
	onLaunchMessageId     internalMessageId = "launch"
	onKillMessageId       internalMessageId = "kill"
	onPreRestartMessageId internalMessageId = "pre_restart"
	onRestartMessageId    internalMessageId = "restart"
	onWatchMessageId      internalMessageId = "watch"
	onUnwatchMessageId    internalMessageId = "unwatch"
	onWatchEndMessageId   internalMessageId = "watch_end"
)

var (
	internalMessageProviders = map[internalMessageId]provider.Provider[internalMessage]{
		onLaunchMessageId:     provider.FN[internalMessage](func() internalMessage { return new(OnLaunch) }),
		onKillMessageId:       provider.FN[internalMessage](func() internalMessage { return new(OnKill) }),
		onPreRestartMessageId: provider.FN[internalMessage](func() internalMessage { return new(OnPreRestart) }),
		onRestartMessageId:    provider.FN[internalMessage](func() internalMessage { return new(OnRestart) }),
		onWatchMessageId:      provider.FN[internalMessage](func() internalMessage { return new(onWatch) }),
		onUnwatchMessageId:    provider.FN[internalMessage](func() internalMessage { return new(onUnwatch) }),
		onWatchEndMessageId:   provider.FN[internalMessage](func() internalMessage { return new(OnWatchEnd) }),
	}
)

func provideInternalMessageInstance(id internalMessageId) internalMessage {
	if p, exist := internalMessageProviders[id]; exist {
		return p.Provide()
	}
	return nil
}

var (
	onLaunchInstance = &OnLaunch{}
)

// OnLaunch 表示 Actor 启动消息。
//
// 当 Actor 被创建并准备开始处理消息时，会收到此消息。
// 这是 Actor 生命周期中的第一个消息，通常用于初始化操作。
type OnLaunch struct{}

func (m *OnLaunch) marshal() (internalMessageId, []byte) {
	return onLaunchMessageId, nil
}

func (m *OnLaunch) unmarshal(b []byte) {
	return
}

// OnKill 表示 Actor 终止消息。
//
// 当需要停止一个 Actor 时，会向其发送此消息。
// Actor 收到此消息后应该进行清理工作并准备停止。
//
// 字段说明：
//   - operator: 发起停止操作的 Actor
//   - reason: 停止的原因说明
//   - poison: 是否为优雅停止（true）还是强制停止（false）
//   - applied: 内部标记，防止重复应用
//
// 在此消息中如果发生致命错误，将会跳过监管策略来确保生命周期的正常结束。
type OnKill struct {
	operator ActorRef // 操作者
	reason   []string // 停止原因
	poison   bool     // 是否为优雅停止
	applied  bool     // 是否已应用，避免将自身的停止信号传递给其他 Actor 导致错误的行为
}

func newOnKill(operator ActorRef, poison bool, reason []string) *OnKill {
	return &OnKill{
		operator: operator,
		poison:   poison,
		reason:   reason,
	}
}

func (m *OnKill) marshal() (internalMessageId, []byte) {
	return onKillMessageId, newWriterCapacity(32).
		writeBool(m.poison).
		writeBool(m.applied).
		writeStrings(m.reason).
		writeString(m.operator.GetAddress()).
		writeString(m.operator.GetPath()).
		bytes()
}

func (m *OnKill) unmarshal(b []byte) {
	var address string
	var path string
	newReader(b).
		readBoolTo(&m.poison).
		readBoolTo(&m.applied).
		readStringsTo(&m.reason).
		readWith(func(r *reader) {
			r.readStringTo(&address)
			r.readStringTo(&path)
			m.operator = NewActorRef(address, path)
		})
}

// IsPoison 返回是否为优雅停止。
//
// 返回值：
//   - true: 优雅停止，Actor 会等待当前消息处理完成后停止
//   - false: 强制停止，Actor 会立即停止
func (m *OnKill) IsPoison() bool {
	return m.poison
}

// Reason 返回停止的原因说明。
func (m *OnKill) Reason() []string {
	return m.reason
}

// OnKilled 表示 Actor 已终止的通知消息。
//
// 当一个 Actor 完成停止过程后，会向其父 Actor 发送此消息。
// 这是 Actor 生命周期管理的重要组成部分。
//
// 字段说明：
//   - operator: 发起停止操作的 Actor
//   - ref: 已停止的 Actor 引用
//   - reason: 停止的原因说明
//   - poison: 是否为优雅停止
type OnKilled struct {
	operator ActorRef // 操作者
	ref      ActorRef // 被停止的 Actor
	reason   []string // 停止原因
	poison   bool     // 是否为优雅停止
}

func newOnKilled(operator, ref ActorRef, poison bool, reason []string) *OnKilled {
	return &OnKilled{
		operator: operator,
		ref:      ref,
		poison:   poison,
		reason:   reason,
	}
}

func (m *OnKilled) marshal() (internalMessageId, []byte) {
	return onKillMessageId, newWriterCapacity(32).
		writeBool(m.poison).
		writeStrings(m.reason).
		writeString(m.ref.GetAddress()).
		writeString(m.ref.GetPath()).
		writeString(m.operator.GetAddress()).
		writeString(m.operator.GetPath()).
		bytes()
}

func (m *OnKilled) unmarshal(b []byte) {
	var address string
	var path string
	newReader(b).
		readBoolTo(&m.poison).
		readStringsTo(&m.reason).
		readWith(func(r *reader) {
			r.readStringTo(&address)
			r.readStringTo(&path)
			m.ref = NewActorRef(address, path)
		}).
		readWith(func(r *reader) {
			r.readStringTo(&address)
			r.readStringTo(&path)
			m.operator = NewActorRef(address, path)
		})
}

// Ref 返回已停止的 Actor 引用。
func (m *OnKilled) Ref() ActorRef {
	return m.ref
}

// IsPoison 返回是否为优雅停止。
func (m *OnKilled) IsPoison() bool {
	return m.poison
}

// Reason 返回停止的原因说明。
func (m *OnKilled) Reason() []string {
	return m.reason
}

// OnPreRestart 表示 Actor 重启前的准备消息。
//
// 当监管者决定重启一个 Actor 时，会先发送此消息。
// Actor 收到此消息后应该进行重启前的清理工作。
//
// 当此消息中发生致命错误时，将会被监管策略进行正常拦截，如若重启，并计入连续重启次数。
type OnPreRestart struct {
}

func (m *OnPreRestart) marshal() (internalMessageId, []byte) {
	return onPreRestartMessageId, nil
}

func (m *OnPreRestart) unmarshal(b []byte) {
	return
}

// OnRestart 表示 Actor 重启消息。
//
// 当 Actor 完成重启准备工作后，会收到此消息开始重新初始化。
// 这标志着 Actor 重启过程的完成，可以重新开始处理用户消息。
//
// 当此消息中发生致命错误时，将会被监管策略进行正常拦截，如若重启，并计入连续重启次数。
type OnRestart struct {
}

func (m *OnRestart) marshal() (internalMessageId, []byte) {
	return onRestartMessageId, nil
}

func (m *OnRestart) unmarshal(b []byte) {
	return
}

// onWatch 表示 Actor 开始被监视的消息。
//
// 当一个 Actor 开始被监视时，会收到此消息。
// 这标志着 Actor 的生命周期开始被监视，在生命周期结束时，将会告知监视者。
type onWatch struct{}

func (m *onWatch) marshal() (internalMessageId, []byte) {
	return onWatchMessageId, nil
}

func (m *onWatch) unmarshal(b []byte) {
	return
}

// onUnwatch 表示 Actor 停止被监视的消息。
type onUnwatch struct{}

func (m *onUnwatch) marshal() (internalMessageId, []byte) {
	return onUnwatchMessageId, nil
}

func (m *onUnwatch) unmarshal(b []byte) {
	return
}

// OnWatchEnd 表示 Actor 生命周期停止的消息。
//
// 当一个 Actor 生命周期停止时，会向其监视者发送此消息。
// 这标志着 Actor 生命周期已经结束，监视者可以进行相应的清理工作。
type OnWatchEnd struct {
	ref    ActorRef // 被监视的 Actor
	reason []string // 停止原因
}

func (m *OnWatchEnd) marshal() (internalMessageId, []byte) {
	return onWatchEndMessageId, newWriterCapacity(32).
		writeStrings(m.reason).
		writeString(m.ref.GetAddress()).
		writeString(m.ref.GetPath()).
		bytes()
}

func (m *OnWatchEnd) unmarshal(b []byte) {
	var address string
	var path string
	newReader(b).
		readStringsTo(&m.reason).
		readWith(func(r *reader) {
			r.readStringTo(&address)
			r.readStringTo(&path)
			m.ref = NewActorRef(address, path)
		})
}
