// Package processor_test 提供了 RPC 功能的使用示例。
package processor_test

import (
    "errors"
    "fmt"
    "testing"

    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/vivid/engine/v1/internal/processor"
)

// TestRegistryRPCServerLifecycle 测试注册表和 RPC 服务器的生命周期管理
func TestRegistryRPCServerLifecycle(t *testing.T) {
    // 创建不带 RPC 服务器的注册表
    registry := processor.NewRegistryWithOptions(
        processor.WithLogger(log.GetDefault()),
    )

    // 测试启动不存在的 RPC 服务器
    if err := registry.StartRPCServer(); err != nil {
        t.Errorf("StartRPCServer should not return error for nil RPC server, got: %v", err)
    }

    // 测试关闭注册表
    if err := registry.Shutdown(registry.GetUnitIdentifier()); err != nil {
        t.Errorf("Registry shutdown failed: %v", err)
    }

    // 测试关闭后启动 RPC 服务器
    if err := registry.StartRPCServer(); !errors.Is(err, processor.ErrRegistryShutdown) {
        t.Errorf("Expected ErrRegistryShutdown, got: %v", err)
    }
}

// BenchmarkRegistryWithRPCConcurrency 测试注册表在高并发场景下的性能
func BenchmarkRegistryWithRPCConcurrency(b *testing.B) {
    registry := processor.NewRegistryWithOptions(
        processor.WithLogger(log.GetDefault()),
        processor.WithDaemon(&TestUnit{}),
    )
    defer func() {
        _ = registry.Shutdown(registry.GetUnitIdentifier())
    }()

    // 注册多个处理单元
    for i := 0; i < 100; i++ {
        unitId := registry.GetUnitIdentifier().Branch(fmt.Sprintf("service/test%d", i))
        _ = registry.RegisterUnit(unitId, &TestUnit{})
    }

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // 模拟获取远程单元（实际会回退到守护单元）
            id := processor.NewCacheUnitIdentifier("remote-server:8080", "/service/remote")
            _, err := registry.GetUnit(id)
            if err != nil {
                b.Errorf("GetUnit failed: %v", err)
            }
        }
    })
}
