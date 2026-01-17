package ves

import (
	"github.com/kercylan98/vivid"
)

// RemotingServerStartedEvent 表示远程服务器启动成功的事件。
//
// 该事件在 ServerActor 成功启动 TCP 监听并开始接受连接时发布。
// 此时服务器已准备好接受来自远程节点的连接请求。
//
// 使用场景：
//   - 监控远程服务器的启动状态
//   - 实现服务注册和发现机制
//   - 记录服务器启动日志和指标
type RemotingServerStartedEvent struct {
	// BindAddr 服务器绑定的本地监听地址（如 "0.0.0.0:8080"）
	BindAddr string
	// AdvertiseAddr 对外宣称的服务地址（如 "public.ip:port"），用于服务注册和远程节点发现
	AdvertiseAddr string
	// ServerRef 远程服务器 Actor 的引用
	ServerRef vivid.ActorRef
}

// RemotingServerStoppedEvent 表示远程服务器停止的事件。
//
// 该事件在 ServerActor 被终止并完全关闭所有连接和监听器后发布。
// 此时服务器已不再接受新的连接请求，所有现有连接也已关闭。
//
// 使用场景：
//   - 监控远程服务器的停止状态
//   - 实现服务注销机制
//   - 记录服务器停止日志和指标
type RemotingServerStoppedEvent struct {
	// BindAddr 服务器绑定的本地监听地址
	BindAddr string
	// AdvertiseAddr 对外宣称的服务地址
	AdvertiseAddr string
	// ServerRef 远程服务器 Actor 的引用
	ServerRef vivid.ActorRef
}

// RemotingConnectionEstablishedEvent 表示远程连接建立成功的事件。
//
// 该事件在 TCP 连接成功建立并完成握手后发布。
// 此时连接已准备好进行消息传输，无论是作为客户端主动连接还是作为服务器接受连接。
//
// 使用场景：
//   - 监控远程连接的数量和状态
//   - 实现连接池管理和统计
//   - 记录连接建立日志和指标
//   - 实现连接级别的监控和告警
type RemotingConnectionEstablishedEvent struct {
	// ConnectionRef 连接 Actor 的引用
	ConnectionRef vivid.ActorRef
	// RemoteAddr 远程节点的地址（连接的远程端地址）
	RemoteAddr string
	// LocalAddr 本地节点的地址（连接的本地端地址）
	LocalAddr string
	// AdvertiseAddr 远程节点对外宣称的服务地址
	AdvertiseAddr string
	// IsClient 是否为客户端主动建立的连接（true 表示客户端连接，false 表示服务器接受的连接）
	IsClient bool
}

// RemotingConnectionClosedEvent 表示远程连接关闭的事件。
//
// 该事件在 TCP 连接被关闭时发布，无论是正常关闭还是异常断开。
// 连接关闭可能由多种原因导致：网络故障、远程节点关闭、主动断开等。
//
// 使用场景：
//   - 监控连接的生命周期和关闭原因
//   - 实现连接重连机制
//   - 记录连接关闭日志和指标
//   - 分析连接稳定性和故障模式
type RemotingConnectionClosedEvent struct {
	// ConnectionRef 已关闭的连接 Actor 的引用
	ConnectionRef vivid.ActorRef
	// RemoteAddr 远程节点的地址
	RemoteAddr string
	// LocalAddr 本地节点的地址
	LocalAddr string
	// AdvertiseAddr 远程节点对外宣称的服务地址
	AdvertiseAddr string
	// IsClient 是否为客户端连接
	IsClient bool
	// Reason 连接关闭的原因描述
	Reason string
}

// RemotingConnectionFailedEvent 表示远程连接建立失败的事件。
//
// 该事件在尝试建立 TCP 连接失败时发布，通常发生在客户端主动连接远程节点时。
// 连接失败可能由多种原因导致：网络不可达、远程节点未启动、防火墙阻止等。
//
// 使用场景：
//   - 监控连接失败率和失败原因
//   - 实现连接重试机制和退避策略
//   - 记录连接失败日志和指标
//   - 实现故障告警和通知机制
type RemotingConnectionFailedEvent struct {
	// RemoteAddr 尝试连接的远程节点地址
	RemoteAddr string
	// AdvertiseAddr 远程节点对外宣称的服务地址
	AdvertiseAddr string
	// Error 连接失败的错误信息
	Error error
	// RetryCount 当前重试次数（如果适用）
	RetryCount int
}

// RemotingMessageSentEvent 表示远程消息发送成功的事件。
//
// 该事件在消息成功序列化并写入到远程连接后发布。
// 注意：此事件仅表示消息已发送到网络层，不保证远程节点已接收或处理。
//
// 使用场景：
//   - 监控远程消息的发送量和频率
//   - 实现消息发送的统计和指标收集
//   - 记录消息发送日志用于调试和追踪
type RemotingMessageSentEvent struct {
	// ConnectionRef 发送消息使用的连接 Actor 的引用
	ConnectionRef vivid.ActorRef
	// RemoteAddr 目标远程节点的地址
	RemoteAddr string
	// MessageType 发送的消息类型名称
	MessageType string
	// MessageSize 消息的字节大小
	MessageSize int
}

// RemotingMessageReceivedEvent 表示远程消息接收成功的事件。
//
// 该事件在从远程连接成功接收并反序列化消息后发布。
// 此时消息已从网络层读取并准备投递到目标 Actor 的邮箱。
//
// 使用场景：
//   - 监控远程消息的接收量和频率
//   - 实现消息接收的统计和指标收集
//   - 记录消息接收日志用于调试和追踪
//   - 实现消息路由和分发监控
type RemotingMessageReceivedEvent struct {
	// ConnectionRef 接收消息使用的连接 Actor 的引用
	ConnectionRef vivid.ActorRef
	// RemoteAddr 发送消息的远程节点地址
	RemoteAddr string
	// MessageType 接收的消息类型名称
	MessageType string
	// MessageSize 消息的字节大小
	MessageSize int
	// ReceiverPath 目标接收者 Actor 的路径
	ReceiverPath string
}

// RemotingMessageSendFailedEvent 表示远程消息发送失败的事件。
//
// 该事件在消息发送过程中发生错误时发布，可能的原因包括：
// 序列化失败、网络写入错误、连接已关闭等。
//
// 使用场景：
//   - 监控消息发送失败率和失败原因
//   - 实现消息发送重试机制
//   - 记录发送失败日志和指标
//   - 实现故障告警和通知机制
type RemotingMessageSendFailedEvent struct {
	// ConnectionRef 发送消息使用的连接 Actor 的引用（如果连接存在）
	ConnectionRef vivid.ActorRef
	// RemoteAddr 目标远程节点的地址
	RemoteAddr string
	// MessageType 发送失败的消息类型名称
	MessageType string
	// Error 发送失败的错误信息
	Error error
}

// RemotingMessageDecodeFailedEvent 表示远程消息解码失败的事件。
//
// 该事件在从远程连接接收消息但反序列化失败时发布。
// 解码失败可能由多种原因导致：消息格式不匹配、编解码器错误、数据损坏等。
//
// 使用场景：
//   - 监控消息解码失败率和失败原因
//   - 诊断编解码器兼容性问题
//   - 记录解码失败日志和指标
//   - 实现数据完整性监控和告警
type RemotingMessageDecodeFailedEvent struct {
	// ConnectionRef 接收消息使用的连接 Actor 的引用
	ConnectionRef vivid.ActorRef
	// RemoteAddr 发送消息的远程节点地址
	RemoteAddr string
	// MessageSize 尝试解码的消息字节大小
	MessageSize int
	// Error 解码失败的错误信息
	Error error
}
