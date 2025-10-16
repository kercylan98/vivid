package registry

import (
	"fmt"

	"github.com/kercylan98/vivid/pkg/runtime"
	"github.com/puzpuzpuz/xsync/v3"
)

var _ runtime.AddressingRegistry = (*AddressingRegistry)(nil)

func NewAddressingRegistryWithConfigurators(configurators ...AddressingRegistryConfigurator) runtime.AddressingRegistry {
	var config = NewAddressingRegistryConfiguration()
	for _, c := range configurators {
		c.Configure(config)
	}
	return NewAddressingRegistryFromConfig(config)
}

func NewAddressingRegistryWithOptions(options ...AddressingRegistryOption) runtime.AddressingRegistry {
	var config = NewAddressingRegistryConfiguration()
	for _, option := range options {
		option(config)
	}
	return NewAddressingRegistryFromConfig(config)
}

func NewAddressingRegistryFromConfig(config *AddressingRegistryConfiguration) runtime.AddressingRegistry {
	if err := config.validate(); err != nil {
		panic(err)
	}

	return &AddressingRegistry{
		config:    *config,
		processes: xsync.NewMapOf[string, runtime.Process](),
	}
}

type AddressingRegistry struct {
	config    AddressingRegistryConfiguration       // 配置
	processes *xsync.MapOf[string, runtime.Process] // 进程映射表，key 为地址，value 为进程
}

// Find implements runtime.AddressingRegistry.
func (m *AddressingRegistry) Find(id *runtime.ProcessID) (runtime.Process, error) {
	cache, ok := id.Get()
	if ok {
		return cache, nil
	}

	// 远程进程
	if m.config.Address != id.Address() {
		return m.fromBroker(id)
	}

	process, ok := m.processes.Load(id.Path())
	if !ok {
		return nil, runtime.ErrProcessNotFound
	}
	return process, nil
}

// Register implements runtime.AddressingRegistry.
func (m *AddressingRegistry) Register(id *runtime.ProcessID, process runtime.Process) (runtime.Process, error) {
	current, ok := m.processes.LoadOrStore(id.Path(), process)
	if ok && current != process {
		return nil, runtime.ErrProcessAlreadyExists
	}
	return current, nil
}

// Unregister implements runtime.AddressingRegistry.
func (m *AddressingRegistry) Unregister(id *runtime.ProcessID) error {
	m.processes.Delete(id.Path())
	return nil
}

func (m *AddressingRegistry) fromBroker(id *runtime.ProcessID) (runtime.Process, error) {
	serializer := m.config.BrokerSerializerProvider.Provide(id)
	if serializer == nil {
		serializer = m.config.Serializer
	}

	writer := m.config.BrokerWriterProvider.Provide(id)
	if writer == nil {
		return nil, fmt.Errorf("%w: %s, broker writer not found", runtime.ErrBadProcess, id)
	}
	broker := newBroker(id, serializer, writer)

	return broker, nil
}
