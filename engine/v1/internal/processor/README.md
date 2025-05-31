# Processor 包

Processor 包提供了 Vivid 引擎 v1 的处理单元注册、管理和路由功能。它是构建分布式处理系统的核心组件，支持高并发、可扩展的消息处理架构。

## 特性

- 🚀 **高性能**：基于无锁数据结构，支持高并发访问
- 🔒 **并发安全**：所有操作都是线程安全的
- 🎯 **灵活配置**：支持多种配置模式和选项
- 🔄 **生命周期管理**：完整的单元初始化和关闭流程
- 📍 **路径路由**：层次化的单元组织和路由
- 💾 **智能缓存**：内置缓存机制提升性能
- 🛡️ **容错设计**：守护单元提供回退机制

## 快速开始

### 基础用法

```go
package main

import (
    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/vivid/engine/v1/internal/processor"
)

// 定义你的处理单元
type MyUnit struct {
    name string
}

func (u *MyUnit) Handle(sender processor.UnitIdentifier, message any) {
    log.Info("Received message", "from", sender.GetPath(), "message", message)
}

func main() {
    // 创建注册表
    registry := processor.NewRegistryWithOptions(
        processor.WithLogger(log.GetDefault()),
    )
    
    // 注册处理单元
    unit := &MyUnit{name: "example"}
    id := registry.GetUnitIdentifier().Branch("myservice")
    registry.RegisterUnit(id, unit)
    
    // 获取并使用处理单元
    cacheId := processor.NewCacheUnitIdentifier("localhost", "/myservice")
    if retrievedUnit, err := registry.GetUnit(cacheId); err == nil {
        retrievedUnit.Handle(id, "Hello, World!")
    }
    
    // 优雅关闭
    registry.Shutdown(registry.GetUnitIdentifier())
}
```

## API 文档

### Registry 接口

注册表是管理所有处理单元的核心组件。

#### 创建注册表

```go
// 使用选项模式（推荐）
registry := processor.NewRegistryWithOptions(
    processor.WithLogger(myLogger),
    processor.WithUnitIdentifier(processor.NewUnitIdentifier("localhost", "/")),
)

// 使用配置器模式
registry := processor.NewRegistryWithConfigurators(
    processor.RegistryConfiguratorFN(func(c *processor.RegistryConfiguration) {
        c.WithLogger(myLogger).WithUnitIdentifier(myIdentifier)
    }),
)

// 从配置对象创建
config := processor.NewRegistryConfiguration(processor.WithLogger(myLogger))
registry := processor.NewRegistryFromConfig(config)
```

#### 主要方法

| 方法                                 | 描述      |
|------------------------------------|---------|
| `RegisterUnit(id, unit)`           | 注册处理单元  |
| `UnregisterUnit(operator, target)` | 注销处理单元  |
| `GetUnit(id)`                      | 获取处理单元  |
| `SetDaemon(unit)`                  | 设置守护单元  |
| `GetDaemon()`                      | 获取守护单元  |
| `Shutdown(operator)`               | 关闭注册表   |
| `UnitCount()`                      | 获取单元数量  |
| `IsShutdown()`                     | 检查是否已关闭 |

### Unit 接口

处理单元是执行业务逻辑的基本组件。

```go
type Unit interface {
    Handle(sender UnitIdentifier, message any)
}
```

#### 扩展接口

**UnitInitializer** - 支持自动初始化：

```go
type UnitInitializer interface {
    Unit
    Init()
}
```

**UnitCloser** - 支持优雅关闭：

```go
type UnitCloser interface {
    Unit
    Close(operator UnitIdentifier)
    Closed() bool
}
```

### UnitIdentifier 接口

单元标识符用于唯一标识和定位处理单元。

```go
type UnitIdentifier interface {
    GetAddress() string
    GetPath() string
    Branch(path string) UnitIdentifier
}
```

#### 缓存标识符

```go
type CacheUnitIdentifier interface {
    UnitIdentifier
    LoadCache() Unit
    StoreCache(unit Unit)
    ClearCache()
}
```

## 使用示例

### 实现完整的处理单元

```go
type ServiceUnit struct {
    name     string
    active   bool
    shutdown bool
}

// 实现 Unit 接口
func (s *ServiceUnit) Handle(sender processor.UnitIdentifier, message any) {
    if s.shutdown {
        return
    }
    
    switch msg := message.(type) {
    case string:
        log.Info("Service received text", "service", s.name, "message", msg)
    case map[string]any:
        log.Info("Service received data", "service", s.name, "data", msg)
    default:
        log.Warn("Unknown message type", "type", fmt.Sprintf("%T", msg))
    }
}

// 实现 UnitInitializer 接口
func (s *ServiceUnit) Init() {
    s.active = true
    log.Info("Service initialized", "service", s.name)
}

// 实现 UnitCloser 接口
func (s *ServiceUnit) Close(operator processor.UnitIdentifier) {
    s.shutdown = true
    s.active = false
    log.Info("Service closed", "service", s.name, "by", operator.GetPath())
}

func (s *ServiceUnit) Closed() bool {
    return s.shutdown
}
```

### 设置守护单元

```go
type DaemonUnit struct{}

func (d *DaemonUnit) Handle(sender processor.UnitIdentifier, message any) {
    log.Warn("Message handled by daemon unit", 
        "sender", sender.GetPath(), 
        "message", message)
}

// 设置守护单元
daemon := &DaemonUnit{}
registry.SetDaemon(daemon)
```

### 层次化单元组织

```go
// 创建服务层次结构
userService := &ServiceUnit{name: "user-service"}
orderService := &ServiceUnit{name: "order-service"}
paymentService := &ServiceUnit{name: "payment-service"}

// 注册到不同路径
registry.RegisterUnit(registry.GetUnitIdentifier().Branch("api/user"), userService)
registry.RegisterUnit(registry.GetUnitIdentifier().Branch("api/order"), orderService)
registry.RegisterUnit(registry.GetUnitIdentifier().Branch("api/payment"), paymentService)

// 创建子服务
userAuthService := &ServiceUnit{name: "user-auth"}
registry.RegisterUnit(registry.GetUnitIdentifier().Branch("api/user/auth"), userAuthService)
```

## 配置选项

### 注册表配置

| 选项                       | 描述       | 默认值                |
|--------------------------|----------|--------------------|
| `WithLogger(logger)`     | 设置日志记录器  | `log.GetDefault()` |
| `WithUnitIdentifier(id)` | 设置根单元标识符 | `localhost:/`      |

### 配置示例

```go
// 自定义配置
config := processor.NewRegistryConfiguration(
    processor.WithLogger(myCustomLogger),
    processor.WithUnitIdentifier(processor.NewUnitIdentifier("192.168.1.100", "/myapp")),
)

registry := processor.NewRegistryFromConfig(config)
```

## 错误处理

包定义了以下错误类型：

| 错误                         | 描述      |
|----------------------------|---------|
| `ErrUnitIdentifierInvalid` | 单元标识符无效 |
| `ErrUnitInvalid`           | 处理单元无效  |
| `ErrUnitAlreadyExists`     | 处理单元已存在 |
| `ErrUnitNotFound`          | 处理单元未找到 |
| `ErrDaemonUnitNotSet`      | 守护单元未设置 |
| `ErrRegistryShutdown`      | 注册表已关闭  |

### 错误处理示例

```go
if err := registry.RegisterUnit(id, unit); err != nil {
    if errors.Is(err, processor.ErrUnitAlreadyExists) {
        log.Warn("Unit already registered", "path", id.GetPath())
    } else {
        log.Error("Failed to register unit", "error", err)
    }
}
```

## 最佳实践

### 1. 单元设计原则

- **单一职责**：每个单元只处理特定类型的消息
- **无状态或可恢复**：避免依赖内部状态，或确保状态可恢复
- **实现生命周期接口**：实现 `UnitInitializer` 和 `UnitCloser` 进行资源管理

### 2. 路径规划

```go
// 好的路径设计
/api/user          // 用户服务
/api/user/auth     // 用户认证子服务
/api/order         // 订单服务
/worker/email      // 邮件工作单元
/worker/notification // 通知工作单元
```

### 3. 性能优化

- 使用 `CacheUnitIdentifier` 提升重复访问性能
- 合理设置守护单元避免查找失败
- 批量操作时考虑并发限制

### 4. 错误处理

- 始终检查 `GetUnit` 的返回错误
- 使用 `errors.Is()` 进行错误类型判断
- 在单元中实现优雅的错误处理

## 线程安全

所有 Registry 方法都是线程安全的，可以安全地在多个 goroutine 中并发调用。单元的 `Handle` 方法需要由实现者确保线程安全。

## 性能考虑

- **并发读取**：支持高并发的单元查找操作
- **缓存机制**：`CacheUnitIdentifier` 减少重复查找开销
- **原子操作**：守护单元管理使用无锁原子操作
- **内存效率**：使用高效的并发安全数据结构

## 许可证

此包遵循项目的许可证条款。 