package persistence

func NewState(persistenceId string, repository Repository) *State {
	return &State{
		id:         persistenceId,
		repository: repository,
	}
}

type State struct {
	id         string     // 状态 ID
	repository Repository // 仓库
	snapshot   Snapshot   // 全量快照
	events     []Event    // 事件
}

// Update 记录状态更新事件
func (s *State) Update(event Event) {
	s.events = append(s.events, event)
}

// SaveSnapshot 保存快照
func (s *State) SaveSnapshot(snapshot Snapshot) {
	s.snapshot = snapshot
	s.events = nil
}

// Persist 保存状态
func (s *State) Persist() error {
	return s.repository.Save(s.id, s.snapshot, s.events)
}

// Load 读取状态
func (s *State) Load() (err error) {
	s.snapshot, s.events, err = s.repository.Load(s.id)
	return
}

// GetSnapshot 获取快照
func (s *State) GetSnapshot() Snapshot {
	return s.snapshot
}

// GetEvents 获取事件
func (s *State) GetEvents() []Event {
	return s.events
}
