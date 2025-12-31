# Vivid

[![Go Reference](https://pkg.go.dev/badge/github.com/kercylan98/vivid.svg)](https://pkg.go.dev/github.com/kercylan98/vivid)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Vivid 是一个高性能、类型安全的 Go 语言 Actor 模型实现库，提供了完整的 Actor 系统、消息传递、远程通信、监督策略等核心功能，帮助开发者构建可扩展、高并发的分布式应用。

## 特性

- **完整的 Actor 模型实现**：提供 Actor 系统、Actor 上下文、Actor 引用等核心抽象
- **灵活的消息传递**：支持 Tell（Fire-and-Forget）和 Ask（Request-Response）两种消息模式
- **远程通信支持**：内置 Remoting 功能，支持跨网络节点的 Actor 通信
- **监督策略**：提供完善的错误处理和恢复机制，包括重启、停止、恢复、升级等决策
- **行为栈管理**：支持 Actor 行为动态切换和恢复，实现状态机模式
- **生命周期管理**：完整的 Actor 生命周期钩子（启动前、重启前、重启后等）
- **指标收集**：内置 Metrics 支持，可监控系统运行状态
- **事件流**：提供事件流机制，支持系统级事件订阅和发布
- **可扩展邮箱**：支持自定义邮箱实现，满足不同调度需求
- **类型安全**：充分利用 Go 的类型系统，提供类型安全的消息传递

## 安装

使用 Go 模块管理工具安装：

```
go get github.com/kercylan98/vivid
```

## 文档

- **API 文档**：完整的 API 文档请访问 [pkg.go.dev](https://pkg.go.dev/github.com/kercylan98/vivid)
- **Wiki 文档**：详细的使用指南和最佳实践请参考 [Wiki 站点](https://github.com/kercylan98/vivid/wiki)（建设中）

## 要求

- Go 1.25.1 或更高版本

## 许可证

本项目采用 [MIT 许可证](LICENSE)。

## 贡献

欢迎提交 Issue 和 Pull Request！