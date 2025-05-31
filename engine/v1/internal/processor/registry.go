// Package processor 提供了处理单元的注册表功能。
package processor

import (
    "fmt"
    "sync/atomic"

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
    return &registry{
        config: config,
        units:  xsync.NewMapOf[string, Unit](),
    }
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
    GetUnit(id CacheUnitIdentifier) (unit Unit, err error)

    // RegisterUnit 注册处理单元到注册表
    // 如果单元实现了 UnitInitializer 接口，会自动调用初始化方法
    RegisterUnit(id UnitIdentifier, unit Unit) (err error)

    // UnregisterUnit 从注册表注销处理单元，如果单元实现了 UnitCloser 接口，会自动调用关闭方法。
    UnregisterUnit(operator, target UnitIdentifier)

    // SetDaemon 设置守护单元，守护单元用作找不到指定单元时的回退选项。
    SetDaemon(unit Unit)

    // GetDaemon 返回守护单元和是否存在的布尔值。
    GetDaemon() (Unit, bool)

    // Shutdown 关闭注册表
    // 会依次关闭所有注册的处理单元
    Shutdown(operator UnitIdentifier) error

    // IsShutdown 检查注册表是否已关闭
    IsShutdown() bool

    // UnitCount 返回当前注册的处理单元数量
    UnitCount() int
}

// registry 注册表的具体实现。
// 使用并发安全的数据结构来支持高并发访问。
type registry struct {
    config   RegistryConfiguration      // 注册表配置
    units    *xsync.MapOf[string, Unit] // 处理单元映射表，key为路径，value为处理单元
    daemon   atomic.Pointer[Unit]       // 守护单元，使用原子指针保证并发安全
    shutdown atomic.Bool                // 关闭状态标志
}

// Logger 实现 Registry 接口，返回日志记录器。
func (r *registry) Logger() log.Logger {
    return r.config.Logger
}

// GetUnitIdentifier 实现 Registry 接口，返回根单元标识符。
func (r *registry) GetUnitIdentifier() UnitIdentifier {
    return r.config.RootUnitIdentifier
}

// SetDaemon 实现 Registry 接口，设置守护单元。
// 守护单元在找不到指定处理单元时作为回退选项。
func (r *registry) SetDaemon(unit Unit) {
    r.daemon.Store(&unit)
    r.Logger().Debug("set daemon unit")
}

// GetDaemon 实现 Registry 接口，获取守护单元。
// 返回守护单元和是否存在的布尔值。
func (r *registry) GetDaemon() (Unit, bool) {
    if daemon := r.daemon.Load(); daemon != nil {
        return *daemon, true
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
func (r *registry) GetUnit(id CacheUnitIdentifier) (unit Unit, err error) {
    // 检查注册表是否已关闭
    if r.IsShutdown() {
        return nil, ErrRegistryShutdown
    }

    // 如果标识符为 nil，返回守护单元
    if id == nil {
        if daemon, exists := r.GetDaemon(); exists {
            return daemon, nil
        }
        return nil, ErrDaemonUnitNotSet
    }

    // 尝试从缓存加载
    if unit = id.LoadCache(); unit != nil {
        return unit, nil
    }

    path := id.GetPath()

    // TODO: 远程解析逻辑
    // if id.GetAddress() != r.config.localAddress {
    //     return r.createRPCUnit(id)
    // }

    // 从本地注册表查找
    if unit, loaded := r.units.Load(path); loaded {
        // 缓存到标识符中以提高后续访问性能
        id.StoreCache(unit)
        return unit, nil
    }

    // 如果找不到指定单元，返回守护单元
    if daemon, exists := r.GetDaemon(); exists {
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

    r.Logger().Debug("register unit", log.String("path", path))
    return nil
}

// UnregisterUnit 实现 Registry 接口，注销处理单元。
// 支持自动关闭和清理操作。
func (r *registry) UnregisterUnit(operator, target UnitIdentifier) {
    if target == nil {
        return
    }

    path := target.GetPath()
    r.Logger().Debug("unregister unit", log.String("path", path))

    if unit, loaded := r.units.LoadAndDelete(path); loaded {
        // 如果单元支持关闭操作，则调用关闭方法
        if closer := asUnitCloser(unit); closer != nil {
            closer.Close(operator)
        }
    }
}

// Shutdown 实现 Registry 接口，关闭注册表。
// 会依次关闭所有注册的处理单元，并设置关闭状态。
func (r *registry) Shutdown(operator UnitIdentifier) error {
    // 设置关闭状态
    if !r.shutdown.CompareAndSwap(false, true) {
        return ErrRegistryShutdown // 已经关闭
    }

    r.Logger().Info("shutting down registry")

    // 关闭所有注册的处理单元
    r.units.Range(func(path string, unit Unit) bool {
        if closer := asUnitCloser(unit); closer != nil {
            closer.Close(operator)
        }
        r.Logger().Debug("shutdown unit", log.String("path", path))
        return true
    })

    // 清空注册表
    r.units.Clear()

    // 清除守护单元
    r.daemon.Store(nil)

    r.Logger().Info("registry shutdown completed")
    return nil
}
