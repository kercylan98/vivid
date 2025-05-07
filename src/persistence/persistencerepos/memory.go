package persistencerepos

import (
	"github.com/kercylan98/vivid/src/persistence"
	"sync"
)

var _ persistence.Repository = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{
		snapshots: make(map[string]persistence.Snapshot),
		events:    make(map[string][]persistence.Event),
	}
}

type Memory struct {
	snapshots map[string]persistence.Snapshot
	events    map[string][]persistence.Event
	rw        sync.RWMutex
}

func (m *Memory) Save(persistenceId string, snapshot persistence.Snapshot, events []persistence.Event) error {
	m.rw.Lock()
	defer m.rw.Unlock()

	m.snapshots[persistenceId] = snapshot
	m.events[persistenceId] = events
	return nil
}

func (m *Memory) Load(persistenceId string) (persistence.Snapshot, []persistence.Event, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	return m.snapshots[persistenceId], m.events[persistenceId], nil
}
