// Package processor_test 提供了处理器组件的单元测试。
package processor_test

import (
	"errors"
	"fmt"
	processor2 "github.com/kercylan98/vivid/core/vivid/internal/processor"
	"testing"

	"github.com/kercylan98/go-log/log"
)

var (
	_ processor2.Unit            = (*TestUnit)(nil)
	_ processor2.UnitInitializer = (*TestUnit)(nil)
	_ processor2.UnitCloser      = (*TestUnit)(nil)
)

// TestUnit 测试用的处理单元实现
type TestUnit struct {
	init   bool // 是否已初始化
	handle bool // 是否已处理消息
	close  bool // 是否已关闭
}

// Init 实现 UnitInitializer 接口
func (t *TestUnit) Init() {
	t.init = true
}

// Handle 实现 Unit 接口
func (t *TestUnit) HandleUserMessage(sender processor2.UnitIdentifier, message any) {
	t.handle = true
}

func (t *TestUnit) HandleSystemMessage(sender processor2.UnitIdentifier, message any) {
	t.handle = true
}

// Close 实现 UnitCloser 接口
func (t *TestUnit) Close(operator processor2.UnitIdentifier) {
	t.close = true
}

// Closed 实现 UnitCloser 接口
func (t *TestUnit) Closed() bool {
	return t.close
}

// TestRegistryConfigurator 测试用的注册表配置器
type TestRegistryConfigurator struct{}

// Configure 实现 RegistryConfigurator 接口
func (t *TestRegistryConfigurator) Configure(c *processor2.RegistryConfiguration) {
	c.WithLogger(log.GetDefault())
}

// TestNewRegistry 测试注册表的创建
func TestNewRegistry(t *testing.T) {
	// 测试从配置创建
	processor2.NewRegistryFromConfig(processor2.NewRegistryConfiguration(processor2.WithLogger(log.GetDefault())))
	processor2.NewRegistryFromConfig(&processor2.RegistryConfiguration{
		Logger: log.GetDefault(),
	})

	// 测试使用配置器创建
	processor2.NewRegistryWithConfigurators(processor2.RegistryConfiguratorFN(func(c *processor2.RegistryConfiguration) {
		c.WithLogger(log.GetDefault())
		c.Logger = log.GetDefault()
	}))
	processor2.NewRegistryWithConfigurators(new(TestRegistryConfigurator))

	// 测试使用选项创建
	processor2.NewRegistryWithOptions(processor2.WithLogger(log.GetDefault()))
}

// TestRegistry_Logger 测试日志记录器功能
func TestRegistry_Logger(t *testing.T) {
	registry := processor2.NewRegistryFromConfig(processor2.NewRegistryConfiguration())
	if registry.Logger() == nil {
		t.Error("registry logger is nil")
	}
}

// TestRegistry_GetUnit 测试处理单元获取功能
func TestRegistry_GetUnit(t *testing.T) {
	daemonUnit := new(TestUnit)
	registry := processor2.NewRegistryFromConfig(processor2.NewRegistryConfiguration().WithDaemon(daemonUnit))

	// 设置守护单元后，nil 标识符应该返回守护单元
	if unit, err := registry.GetUnit(nil); err != nil || unit != daemonUnit {
		t.Errorf("expected daemon unit, got unit=%v, err=%v", unit, err)
	}

	// 测试不存在的单元，应该返回守护单元
	id := processor2.NewCacheUnitIdentifier("localhost", "/nonexistent")
	if unit, err := registry.GetUnit(id); err != nil || unit != daemonUnit {
		t.Errorf("expected daemon unit for nonexistent path, got unit=%v, err=%v", unit, err)
	}
}

// TestRegistry_RegisterUnit 测试处理单元注册功能
func TestRegistry_RegisterUnit(t *testing.T) {
	registry := processor2.NewRegistryFromConfig(processor2.NewRegistryConfiguration())

	var tests = []struct {
		name       string
		identifier processor2.UnitIdentifier
		unit       processor2.Unit
		err        error
	}{
		{
			name:       "success",
			identifier: registry.GetUnitIdentifier().Branch("test"),
			unit:       new(TestUnit),
		},
		{
			name:       "already exists",
			identifier: registry.GetUnitIdentifier().Branch("test"),
			unit:       new(TestUnit),
			err:        processor2.ErrUnitAlreadyExists,
		},
		{
			name:       "nil unit",
			identifier: registry.GetUnitIdentifier().Branch("nil"),
			err:        processor2.ErrUnitInvalid,
		},
		{
			name: "register not nil unit",
			// 上一个测试由于被拦截，所以这里能够注册成功
			identifier: registry.GetUnitIdentifier().Branch("nil"),
			unit:       new(TestUnit),
		},
		{
			name: "invalid identifier",
			unit: new(TestUnit),
			err:  processor2.ErrUnitIdentifierInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := registry.RegisterUnit(tt.identifier, tt.unit); !errors.Is(err, tt.err) {
				t.Errorf("registry.RegisterUnit() error = %v, wantErr %v", err, tt.err)
			}
		})
	}
}

// TestRegistry_UnregisterUnit 测试处理单元注销功能
func TestRegistry_UnregisterUnit(t *testing.T) {
	registry := processor2.NewRegistryFromConfig(processor2.NewRegistryConfiguration())

	// 注册一个测试单元
	testUnit := &TestUnit{}
	id := registry.GetUnitIdentifier().Branch("test")
	if err := registry.RegisterUnit(id, testUnit); err != nil {
		t.Fatalf("failed to register unit: %v", err)
	}

	// 验证单元已注册
	if registry.UnitCount() != 1 {
		t.Errorf("expected 1 unit, got %d", registry.UnitCount())
	}

	// 注销单元
	registry.UnregisterUnit(registry.GetUnitIdentifier(), id)

	// 验证单元已注销
	if registry.UnitCount() != 0 {
		t.Errorf("expected 0 units after unregister, got %d", registry.UnitCount())
	}

	// 验证单元的 Close 方法被调用
	if !testUnit.Closed() {
		t.Error("expected unit to be closed after unregister")
	}
}

// TestRegistry_Shutdown 测试注册表关闭功能
func TestRegistry_Shutdown(t *testing.T) {
	daemonUnit := &TestUnit{}
	registry := processor2.NewRegistryFromConfig(processor2.NewRegistryConfiguration().WithDaemon(daemonUnit))

	// 注册一些测试单元
	testUnits := []*TestUnit{
		{}, {}, {},
	}
	for i, unit := range testUnits {
		id := registry.GetUnitIdentifier().Branch(fmt.Sprintf("test%d", i))
		if err := registry.RegisterUnit(id, unit); err != nil {
			t.Fatalf("failed to register unit %d: %v", i, err)
		}
	}

	// 验证注册表状态
	if registry.IsShutdown() {
		t.Error("expected registry not to be shutdown")
	}
	if registry.UnitCount() != 3 {
		t.Errorf("expected 3 units, got %d", registry.UnitCount())
	}

	// 执行关闭
	if err := registry.Shutdown(registry.GetUnitIdentifier()); err != nil {
		t.Errorf("unexpected error during shutdown: %v", err)
	}

	// 验证关闭状态
	if !registry.IsShutdown() {
		t.Error("expected registry to be shutdown")
	}
	if registry.UnitCount() != 0 {
		t.Errorf("expected 0 units after shutdown, got %d", registry.UnitCount())
	}

	// 验证所有单元都被关闭
	for i, unit := range testUnits {
		if !unit.Closed() {
			t.Errorf("expected unit %d to be closed", i)
		}
	}

	// 验证关闭后的操作会返回错误
	testUnit := &TestUnit{}
	id := registry.GetUnitIdentifier().Branch("after-shutdown")
	if err := registry.RegisterUnit(id, testUnit); !errors.Is(err, processor2.ErrRegistryShutdown) {
		t.Errorf("expected ErrRegistryShutdown, got %v", err)
	}

	if _, err := registry.GetUnit(processor2.NewCacheUnitIdentifier("localhost", "/test")); !errors.Is(err, processor2.ErrRegistryShutdown) {
		t.Errorf("expected ErrRegistryShutdown, got %v", err)
	}
}
