package vivid

type (
	Message           = any
	internalMessageId = uint16
	internalMessage   interface {
		marshal() (internalMessageId, []byte)
		unmarshal(b []byte)
	}
)

const (
	onLaunchMessageId internalMessageId = iota + 1
	onKillMessageId
	onPreRestartMessageId
	onRestartMessageId
)

var (
	onLaunchInstance = &OnLaunch{}
)

type OnLaunch struct{}

func (m *OnLaunch) marshal() (internalMessageId, []byte, error) {
	return onLaunchMessageId, nil, nil
}

func (m *OnLaunch) unmarshal(b []byte) error {
	return nil
}

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

func (m *OnKill) IsPoison() bool {
	return m.poison
}

func (m *OnKill) Reason() []string {
	return m.reason
}

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

func (m *OnKilled) Ref() ActorRef {
	return m.ref
}

func (m *OnKilled) IsPoison() bool {
	return m.poison
}

func (m *OnKilled) Reason() []string {
	return m.reason
}

type OnPreRestart struct {
}

func (m *OnPreRestart) marshal() (internalMessageId, []byte) {
	return onPreRestartMessageId, nil
}

func (m *OnPreRestart) unmarshal(b []byte) {
}

type OnRestart struct {
}

func (m *OnRestart) marshal() (internalMessageId, []byte) {
	return onRestartMessageId, nil
}

func (m *OnRestart) unmarshal(b []byte) {
}
