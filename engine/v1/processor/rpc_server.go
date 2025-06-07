package processor

import "net"

type RPCServer interface {
	// Serve 服务启动
	Serve(listen net.Listener) error

	// Stop 服务停止
	Stop() error

	// Listen 获取监听新连接建立的通道
	Listen() <-chan RPCConn
}
