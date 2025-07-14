# Vivid Ecology

> 可选扩展组件目录 - 按需引入，保持核心轻量

<!-- 此文档由 go generate 自动生成，请勿手动编辑 -->
<!-- go:generate go run ./tools/docgen --target=ecology --output=README.md -->

## 组件导航

| 组件 | 状态 | 描述 | 文档 |
|------|------|------|------|
| [grpc-server](./grpc-server/) | ✅ 可用 | High-performance gRPC server integration for Vivid Actor Framework | [README](./grpc-server/README.md) |
| [http-gateway](./http-gateway/) | 🚧 开发中 | HTTP to Actor message gateway with RESTful API support | - |

## 快速安装

```bash
# 安装特定组件
go get github.com/kercylan98/vivid/ecology/grpc-server

# 查看组件详细信息
cd ecology/grpc-server && cat README.md
```

## 组件规范

每个组件必须包含：
- `component.yaml` - 组件配置和元数据
- `go.mod` - 独立模块定义
- `README.md` - 组件文档
- `component.go` - 主要实现
- `examples/` - 使用示例

### 组件配置格式

```yaml
component:
  name: grpc-server
  version: v1.0.0
  status: stable
  category: network
  description: High-performance gRPC server integration

author:
  name: Vivid Team
  email: team@vivid.dev

dependencies:
  go:
    - google.golang.org/grpc
    - github.com/kercylan98/vivid/core/vivid

features:
  - Actor-based gRPC service handling
  - Automatic lifecycle management
```
