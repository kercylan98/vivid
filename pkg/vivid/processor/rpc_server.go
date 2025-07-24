package processor

import "net"

// RPCServer 定义了 RPC 服务器的接口。
//
// RPCServer 负责监听网络连接并处理客户端请求。
// 它为 Actor 系统提供了网络服务能力，支持远程 Actor 通信。
//
// 服务器的生命周期：
//  1. 创建服务器实例
//  2. 调用 Serve() 开始监听
//  3. 通过 Listen() 获取新连接
//  4. 处理连接上的消息
//  5. 调用 Stop() 停止服务
type RPCServer interface {
	// Serve 启动服务器并开始监听连接。
	//
	// 此方法会阻塞直到服务器停止或发生错误。
	// 参数 listen 是网络监听器，通常通过 net.Listen() 创建。
	// 返回服务过程中可能出现的错误。
	Serve(listen net.Listener) error

	// Stop 停止服务器。
	//
	// 停止接受新连接并关闭现有连接。
	// 返回停止过程中可能出现的错误。
	Stop() error

	// Listen 获取监听新连接建立的通道。
	//
	// 返回一个只读通道，当有新连接建立时会发送 RPCConn 实例。
	// 调用者应该监听此通道来处理新连接。
	Listen() <-chan RPCConn
}
