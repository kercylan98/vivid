package registry

import (
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

	// 初始化连接池配置
	connectionPool := newConnectionPool(config.ServerConfig.AdvertiseAddress, config.ServerConfig.ConnectionPoolConfig)

	registry := &AddressingRegistry{
		config:         *config,
		processes:      xsync.NewMapOf[string, runtime.Process](),
		connectionPool: connectionPool,
		closeCh:        make(chan struct{}),
	}
	if config.ServerConfig.Server != nil {
		registry.serverHandler = newServerHandler(config.ServerConfig.AdvertiseAddress, registry, config.Serializer)
	}

	return registry
}

type AddressingRegistry struct {
	config         AddressingRegistryConfiguration       // 配置
	processes      *xsync.MapOf[string, runtime.Process] // 进程映射表，key 为路径，value 为进程
	connectionPool *connectionPool                       // 连接池
	closeCh        chan struct{}                         // 关闭信号
	serverHandler  *serverHandler                        // 服务器处理器
}

// Find implements runtime.AddressingRegistry.
func (m *AddressingRegistry) Find(id *runtime.ProcessID) (runtime.Process, error) {
	cache, ok := id.Get()
	if ok {
		return cache, nil
	}

	// 远程进程
	if m.config.ServerConfig.AdvertiseAddress != id.Address() {
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

// Start 启动服务器接收循环（如果配置了服务器）。
func (m *AddressingRegistry) Start() error {
	if m.config.ServerConfig.Server == nil {
		return nil
	}

	go m.acceptLoop()
	return nil
}

// Shutdown 优雅关闭注册表。
func (m *AddressingRegistry) Shutdown() error {
	select {
	case <-m.closeCh:
		return nil // 已经关闭
	default:
		close(m.closeCh)
	}

	// 关闭连接池
	if m.connectionPool != nil {
		m.connectionPool.Close()
	}

	return nil
}

// acceptLoop 循环接受新连接。
func (m *AddressingRegistry) acceptLoop() {
	for {
		select {
		case <-m.closeCh:
			return
		default:
			conn, err := m.config.ServerConfig.Server.Accept()
			if err != nil {
				select {
				case <-m.closeCh:
					return
				default:
					// 记录错误但继续
					continue
				}
			}

			// 为每个连接启动处理 goroutine
			go func() {
				if err := m.serverHandler.HandleConnection(conn); err != nil {
					// 连接处理错误，记录并继续
				}
			}()
		}
	}
}

func (m *AddressingRegistry) fromBroker(id *runtime.ProcessID) (runtime.Process, error) {
	serializer := m.config.Serializer
	if m.config.BrokerSerializerProvider != nil {
		if providedSerializer := m.config.BrokerSerializerProvider.Provide(id); providedSerializer != nil {
			serializer = providedSerializer
		}
	}

	broker := newBroker(id, serializer, m.connectionPool)
	return broker, nil
}
