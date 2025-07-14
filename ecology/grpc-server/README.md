# gRPC Server

gRPC 服务器组件，为 Vivid Actor 系统提供 gRPC 集成。

## 安装

```bash
go get github.com/kercylan98/vivid/ecology/grpc-server
```

## 使用

```go
import (
    "github.com/kercylan98/vivid/core/vivid"
    "github.com/kercylan98/vivid/ecology/grpc-server"
)

// 创建 Actor 系统
system := vivid.NewActorSystemWithOptions()
defer system.Shutdown(true, "程序结束")

// 创建 gRPC 服务器
server := grpcserver.NewServer(
    grpcserver.WithPort(8080),
    grpcserver.WithHost("localhost"),
    grpcserver.WithActorSystem(system),
)

// 启动服务器
if err := server.Start(); err != nil {
    log.Fatal(err)
}
defer server.Stop()
```

## 配置选项

- `WithPort(port int)` - 设置服务器端口
- `WithHost(host string)` - 设置服务器主机
- `WithActorSystem(system vivid.ActorSystem)` - 设置 Actor 系统
- `WithGRPCOptions(opts ...grpc.ServerOption)` - 添加 gRPC 选项

## 示例

查看 [examples](./examples/) 目录获取更多使用示例。