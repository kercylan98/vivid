// Package processor 提供了处理单元的注册表功能。
package processor

import (
	"context"
	"fmt"
	processor3 "github.com/kercylan98/vivid/pkg/vivid/processor"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/puzpuzpuz/xsync/v3"
)

// NewRegistryWithOptions 使用选项模式创建新的注册表实例。
func NewRegistryWithOptions(options ...RegistryOption) Registry {
	return newRegistry(*NewRegistryConfiguration(options...))
}

// NewRegistryWithConfigurators 使用配置器模式创建新的注册表实例。
func NewRegistryWithConfigurators(configurators ...RegistryConfigurator) Registry {
	var config = NewRegistryConfiguration()
	for _, c := range configurators {
		c.Configure(config)
	}
	return newRegistry(*config)
}

// NewRegistryFromConfig 从配置对象创建新的注册表实例。
func NewRegistryFromConfig(config *RegistryConfiguration) Registry {
	return newRegistry(*config)
}

// newRegistry 内部构造函数，创建注册表实例。
// 确保配置的完整性和合理的默认值。
func newRegistry(config RegistryConfiguration) *registry {
	if config.Logger == nil {
		config.Logger = log.GetDefault()
	}

	r := &registry{
		config:    config,
		units:     xsync.NewMapOf[string, Unit](),
		rpcServer: config.RPCServer,
	}

	// 如果配置了 RPC 服务器，设置上下文
	if r.rpcServer != nil {
		r.rootUnitIdentifier = newUnitIdentifier(r.rpcServer.config.AdvertisedAddress, "/")
		r.rpcLock = xsync.NewMapOf[string, *sync.Mutex]()
		r.ctx, r.cancel = context.WithCancel(context.Background())
	} else {
		r.rootUnitIdentifier = newUnitIdentifier(onlyLocalAddress, "/")
	}

	return r
}

// Registry 定义了处理单元注册表的接口。
// 注册表负责管理所有处理单元的生命周期，包括注册、查找、注销等操作。
// 支持本地和远程处理单元的统一管理。
type Registry interface {
	// Logger 返回注册表使用的日志记录器
	Logger() log.Logger

	// GetUnitIdentifier 返回注册表的根单元标识符
	GetUnitIdentifier() UnitIdentifier

	// GetUnit 通过标识符获取处理单元
	// 支持缓存机制和守护单元回退
	GetUnit(id UnitIdentifier) (unit Unit, err error)

	// RegisterUnit 注册处理单元到注册表
	// 如果单元实现了 UnitInitializer 接口，会自动调用初始化方法
	RegisterUnit(id UnitIdentifier, unit Unit) (err error)

	// UnregisterUnit 从注册表注销处理单元，如果单元实现了 UnitCloser 接口，会自动调用关闭方法。
	UnregisterUnit(operator, target UnitIdentifier)

	// StartRPCServer 启动 RPC 服务器
	// 如果配置了 RPC 服务器，则启动服务器监听远程连接
	// 此方法是幂等的，多次调用不会产生副作用
	StartRPCServer() error

	// Shutdown 关闭注册表
	// 会依次关闭所有注册的处理单元和 RPC 服务器
	Shutdown() error

	// IsShutdown 检查注册表是否已关闭
	IsShutdown() bool

	// UnitCount 返回当前注册的处理单元数量
	UnitCount() int
}

// registry 注册表的具体实现。
// 使用并发安全的数据结构来支持高并发访问。
// 支持 RPC 服务器的自动生命周期管理。
type registry struct {
	config             RegistryConfiguration             // 注册表配置
	rootUnitIdentifier UnitIdentifier                    // 根单元标识符
	units              *xsync.MapOf[string, Unit]        // 处理单元映射表，key为路径，value为处理单元
	shutdown           atomic.Bool                       // 关闭状态标志
	rpcServer          *RPCServer                        // RPC 服务器实例
	rpcLock            *xsync.MapOf[string, *sync.Mutex] // RPC 服务器锁，防止并发访问冲突
	ctx                context.Context                   // RPC 服务器上下文
	cancel             context.CancelFunc                // RPC 服务器取消函数
}

// Logger 实现 Registry 接口，返回日志记录器。
func (r *registry) Logger() log.Logger {
	return r.config.Logger
}

// GetUnitIdentifier 实现 Registry 接口，返回根单元标识符。
func (r *registry) GetUnitIdentifier() UnitIdentifier {
	return r.rootUnitIdentifier
}

// getDaemon 实现 Registry 接口，获取守护单元。
// 返回守护单元和是否存在的布尔值。
func (r *registry) getDaemon() (Unit, bool) {
	if r.config.Daemon != nil {
		return r.config.Daemon, true
	}
	return nil, false
}

// IsShutdown 实现 Registry 接口，检查注册表是否已关闭。
func (r *registry) IsShutdown() bool {
	return r.shutdown.Load()
}

// UnitCount 实现 Registry 接口，返回当前注册的处理单元数量。
func (r *registry) UnitCount() int {
	count := 0
	r.units.Range(func(string, Unit) bool {
		count++
		return true
	})
	return count
}

// GetUnit 实现 Registry 接口，通过标识符获取处理单元。
// 查找顺序：缓存 -> 本地注册表 -> 远程解析 -> 守护单元。
func (r *registry) GetUnit(id UnitIdentifier) (unit Unit, err error) {
	// 检查注册表是否已关闭
	if r.IsShutdown() {
		return nil, ErrRegistryShutdown
	}

	// 如果标识符为 nil，返回守护单元
	if id == nil {
		if daemon, exists := r.getDaemon(); exists {
			return daemon, nil
		}
		return nil, ErrDaemonUnitNotSet
	}

	// 尝试从缓存加载
	cache, cached := id.(CacheUnitIdentifier)
	if cached {
		if unit = cache.LoadCache(); unit != nil {
			return unit, nil
		}
	}

	path := id.GetPath()

	// 远程单元解析
	if id.GetAddress() != r.rootUnitIdentifier.GetAddress() {
		return r.fromRPC(id)
	}

	// 从本地注册表查找
	if unit, loaded := r.units.Load(path); loaded {
		// 缓存到标识符中以提高后续访问性能
		if cached {
			cache.StoreCache(unit)
		}
		return unit, nil
	}

	// 如果找不到指定单元，返回守护单元
	if daemon, exists := r.getDaemon(); exists {
		return daemon, nil
	}

	return nil, ErrUnitNotFound
}

// RegisterUnit 实现 Registry 接口，注册处理单元。
// 支持重复注册检查和自动初始化。
func (r *registry) RegisterUnit(id UnitIdentifier, unit Unit) (err error) {
	// 检查注册表是否已关闭
	if r.IsShutdown() {
		return ErrRegistryShutdown
	}

	if id == nil {
		return ErrUnitIdentifierInvalid
	}
	if unit == nil {
		return ErrUnitInvalid
	}

	path := id.GetPath()

	// 使用 LoadOrStore 进行原子性检查和存储
	if existingUnit, loaded := r.units.LoadOrStore(path, unit); loaded {
		// 如果是同一个单元实例，认为是重复注册，直接返回成功
		if existingUnit == unit {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrUnitAlreadyExists, path)
	}

	// 尝试初始化单元
	if initializer := asUnitInitializer(unit); initializer != nil {
		initializer.Init()
	}

	return nil
}

// UnregisterUnit 实现 Registry 接口，注销处理单元。
// 支持自动关闭和清理操作。
func (r *registry) UnregisterUnit(operator, target UnitIdentifier) {
	if target == nil {
		return
	}

	path := target.GetPath()

	if unit, loaded := r.units.LoadAndDelete(path); loaded {
		// 如果单元支持关闭操作，则调用关闭方法
		if closer := asUnitCloser(unit); closer != nil {
			closer.Close(operator)
		}
	}
}

// StartRPCServer 实现 Registry 接口，启动 RPC 服务器。
// 此方法是幂等的，多次调用不会产生副作用。
// 如果未配置 RPC 服务器，此方法将返回 nil。
func (r *registry) StartRPCServer() error {
	if r.IsShutdown() {
		return ErrRegistryShutdown
	}

	if r.rpcServer == nil {
		r.Logger().Debug("no RPC server configured, skipping startup")
		return nil
	}

	r.Logger().Info("starting RPC server")

	// 设置 RPC 服务器的上下文
	if r.ctx != nil {
		r.rpcServer.SetContext(r.ctx)
	}

	// 在独立的 goroutine 中启动 RPC 服务器
	go func() {
		if err := r.rpcServer.Run(); err != nil {
			r.Logger().Error("RPC server error", log.Err(err))
		}
	}()

	r.Logger().Info("RPC server started successfully")
	return nil
}

// fromRPC 处理远程 RPC 连接的获取逻辑。
// 增加了超时机制和更完善的错误处理。
func (r *registry) fromRPC(id UnitIdentifier) (unit Unit, err error) {
	// 使用锁确保同一 ID 的 RPC 连接只被一个协程使用，防止并发访问
	key := id.GetAddress() + id.GetPath()
	lock, _ := r.rpcLock.LoadOrStore(key, &sync.Mutex{})

	// 增加超时机制防止死锁
	done := make(chan struct{})
	go func() {
		lock.Lock()
		close(done)
	}()

	select {
	case <-done:
		defer func() {
			r.rpcLock.Delete(key)
			lock.Unlock()
		}()
	case <-time.After(5 * time.Second): // 5秒超时
		return nil, fmt.Errorf("RPC lock timeout for %s", key)
	}

	// 建立远程单元客户端
	var conn processor3.RPCConn
	if r.config.RPCClientProvider != nil {
		if conn, err = r.config.RPCClientProvider.Provide(id.GetAddress()); err != nil {
			return nil, fmt.Errorf("RPC client provider error: %w", err)
		} else {
			handshake := processor3.NewRPCHandshakeWithAddress(r.rootUnitIdentifier.GetAddress())
			handshakeBuf, err := handshake.Marshal()
			if err != nil {
				return nil, fmt.Errorf("RPC handshake marshal error: %w", err)
			}
			if err = conn.Send(handshakeBuf); err != nil {
				return nil, fmt.Errorf("RPC handshake error: %w", err)
			}
			go r.rpcServer.onConnected(conn)
		}
	} else if r.rpcServer != nil {
		// 尝试查找已有连接，增加空指针检查
		conn = r.rpcServer.GetConn(id.GetAddress())
	}

	if conn == nil {
		if daemon, exists := r.getDaemon(); exists {
			return daemon, nil
		}
		return nil, ErrDaemonUnitNotSet
	}

	rpcUnitConfig := NewRPCUnitConfiguration(WithRPCUnitLogger(r.Logger()))
	if r.config.RPCUnitConfigurator != nil {
		r.config.RPCUnitConfigurator.Configure(rpcUnitConfig)
	}
	unit = NewRPCUnit(id, conn, rpcUnitConfig)

	if cache, ok := id.(CacheUnitIdentifier); ok {
		cache.StoreCache(unit)
	}
	return unit, nil
}

// Shutdown 实现 Registry 接口，关闭注册表。
// 会依次关闭所有注册的处理单元，并关闭 RPC 服务器。
func (r *registry) Shutdown() error {
	// 设置关闭状态
	if !r.shutdown.CompareAndSwap(false, true) {
		return ErrRegistryShutdown // 已经关闭
	}

	r.Logger().Debug("shutting down registry")

	// 清空注册表
	r.units.Clear()

	// 关闭 RPC 服务器
	if r.rpcServer != nil && r.cancel != nil {
		r.Logger().Debug("shutting down RPC server")
		r.rpcServer.Stop()
		r.cancel()
		r.Logger().Debug("RPC server shutdown completed")
	}

	r.Logger().Debug("registry shutdown completed")
	return nil
}
