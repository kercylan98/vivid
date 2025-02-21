package vivid

import "sync"

var (
	_                                  PersistentStorage = (*memoryPersistentStorage)(nil)
	defaultMemoryPersistentStorage     *memoryPersistentStorage
	defaultMemoryPersistentStorageInit sync.Once
)

// PersistentStorage 持久化存储器接口
type PersistentStorage interface {
	// Save 保存持久化数据
	Save(persistentId string, snapshot Message, events []Message) error

	// Load 加载持久化数据
	Load(persistentId string) (snapshot Message, events []Message, err error)
}

// GetMemoryPersistentStorage 创建一个内部提供的基于内存持久化的存储器
func GetMemoryPersistentStorage() PersistentStorage {
	defaultMemoryPersistentStorageInit.Do(func() {
		defaultMemoryPersistentStorage = &memoryPersistentStorage{
			storage: make(map[string][2]Message),
		}
	})
	return defaultMemoryPersistentStorage
}

type memoryPersistentStorage struct {
	storage map[string][2]Message
	rw      sync.RWMutex
}

func (m *memoryPersistentStorage) Save(persistentId string, snapshot Message, events []Message) error {
	m.rw.Lock()
	defer m.rw.Unlock()
	m.storage[persistentId] = [2]Message{snapshot, events}
	return nil
}

func (m *memoryPersistentStorage) Load(persistentId string) (snapshot Message, events []Message, err error) {
	m.rw.RLock()
	defer m.rw.RUnlock()
	if v, ok := m.storage[persistentId]; ok {
		return v[0], v[1].([]Message), nil
	}
	return nil, nil, nil
}
