# GRPC-Net

基于 gRPC 实现的 Vivid ActorSystem 跨网络通讯支持

## 概述

GRPC-Net 是 Vivid Actor 模型框架的网络通信扩展模块，提供了基于 gRPC 的分布式 Actor 通信能力。通过该模块，不同的 ActorSystem 实例可以跨网络进行高效、可靠的消息传递。

## 特性

- **高性能 gRPC 通信**：基于 HTTP/2 协议的双向流式通信
- **Protocol Buffers 序列化**：高效的消息序列化和反序列化
- **透明的网络通信**：对 Actor 代码无侵入的跨网络消息传递
- **自动连接管理**：自动处理网络连接的建立和维护
- **故障恢复**：内置的连接重试和故障恢复机制

## 安装

```bash
go get github.com/kercylan98/vivid/grpc-net
```

## 依赖

- Go 1.24.0+
- github.com/kercylan98/vivid v0.0.2+
- google.golang.org/grpc v1.74.2+
- google.golang.org/protobuf v1.36.6+

## 快速开始

### 1. 定义消息类型

首先，使用 Protocol Buffers 定义消息类型：

```protobuf
// messages.proto
syntax = "proto3";

package messages;
option go_package = "./;messages";

message Message {
    string text = 1;
}
```

生成 Go 代码：

```bash
protoc --go_out=./ --go-grpc_out=./ ./*.proto
```

### 2. 创建分布式 ActorSystem

```go
package main

import (
    "fmt"
    "github.com/kercylan98/vivid/grpc-net/grpcnet"
    "github.com/kercylan98/vivid/pkg/vivid"
    "your-project/messages"
)

func main() {
    // 创建第一个 ActorSystem（监听 19858 端口）
    system1 := vivid.NewActorSystemWithConfigurators(
        vivid.ActorSystemConfiguratorFN(func(c *vivid.ActorSystemConfiguration) {
            c.WithNetwork(grpcnet.NewNetworkConfiguration("127.0.0.1:19858"))
        }),
    )
    
    // 创建第二个 ActorSystem（监听 19859 端口）
    system2 := vivid.NewActorSystemWithConfigurators(
        vivid.ActorSystemConfiguratorFN(func(c *vivid.ActorSystemConfiguration) {
            c.WithNetwork(grpcnet.NewNetworkConfiguration("127.0.0.1:19859"))
        }),
    )
    
    // 在 system1 中创建一个 Actor
    ref := system1.SpawnOf(func() vivid.Actor {
        return vivid.ActorFN(func(context vivid.ActorContext) {
            switch m := context.Message().(type) {
            case *messages.Message:
                fmt.Printf("收到来自 %v 的消息: %s\n", context.Sender(), m.Text)
                context.Reply(m) // 回复相同的消息
            }
        })
    })
    
    // 克隆 ActorRef 以移除本地缓存，强制使用网络通信
    remoteRef := ref.Clone()
    
    // 从 system2 向 system1 中的 Actor 发送消息
    system2.Tell(remoteRef, &messages.Message{Text: "Hello, Remote Actor!"})
    
    // 发送消息并等待回复
    echo, err := vivid.TypedAsk[*messages.Message](
        system2, 
        remoteRef, 
        &messages.Message{Text: "Ping"}
    ).Result()
    if err != nil {
        panic(err)
    }
    fmt.Printf("收到回复: %s\n", echo.Text)
}
```

## API 参考

### NewNetworkConfiguration

```go
func NewNetworkConfiguration(bindAddr string, advertisedAddr ...string) *vivid.ActorSystemNetworkConfiguration
```

创建网络配置，用于配置 ActorSystem 的网络通信能力。

**参数：**
- `bindAddr`: 绑定地址，格式为 "host:port"
- `advertisedAddr`: 可选的广告地址，用于告知其他节点如何连接到当前节点

**返回值：**
- `*vivid.ActorSystemNetworkConfiguration`: 网络配置对象

### 消息序列化

GRPC-Net 使用 Protocol Buffers 进行消息序列化。所有跨网络传输的消息都必须实现 `proto.Message` 接口。

## 架构设计

### 组件结构

```
grpcnet/
├── network.go      # 网络配置入口
├── server.go       # gRPC 服务器实现
├── client.go       # gRPC 客户端连接器
├── conn.go         # 连接抽象层
├── serializer.go   # Protocol Buffers 序列化器
└── internal/
    └── stream/     # gRPC 流定义
```

### 通信流程

1. **连接建立**：客户端通过 gRPC 连接到远程 ActorSystem
2. **消息序列化**：使用 Protocol Buffers 序列化消息
3. **流式传输**：通过 gRPC 双向流传输消息
4. **消息路由**：远程 ActorSystem 将消息路由到目标 Actor
5. **响应处理**：处理回复消息并返回给发送方

## 最佳实践

### 1. 消息设计

- 使用 Protocol Buffers 定义所有跨网络传输的消息
- 保持消息结构简单，避免嵌套过深
- 为消息添加版本字段以支持向后兼容

### 2. 网络配置

- 在生产环境中使用具体的 IP 地址而非 localhost
- 合理配置广告地址，确保其他节点能够正确连接
- 考虑使用负载均衡器进行流量分发

### 3. 错误处理

- 实现适当的超时机制
- 处理网络分区和连接失败的情况
- 使用监督策略处理 Actor 故障

### 4. 性能优化

- 批量发送消息以减少网络开销
- 使用连接池管理多个连接
- 监控网络延迟和吞吐量

## 示例项目

完整的示例代码位于 `example/` 目录中，包含：

- `main.go`: 基本的跨网络通信示例
- `messages/`: Protocol Buffers 消息定义

运行示例：

```bash
cd example
go run main.go
```

## 故障排除

### 常见问题

1. **连接失败**
   - 检查网络配置和防火墙设置
   - 确认目标地址可达
   - 验证端口是否被占用

2. **消息序列化错误**
   - 确保消息类型实现了 `proto.Message` 接口
   - 检查 Protocol Buffers 代码生成是否正确
   - 验证消息类型注册

3. **性能问题**
   - 监控网络延迟和带宽使用
   - 检查消息大小和频率
   - 考虑使用消息批处理

## 贡献

欢迎提交 Issue 和 Pull Request 来改进 GRPC-Net 模块。

## 许可证

本项目采用 MIT 许可证，详见 [LICENSE](../../LICENSE) 文件。

