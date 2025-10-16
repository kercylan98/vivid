package registry

// Connection 表示一个网络连接的抽象接口。
// 该接口定义了连接的基本操作，包括发送、接收和关闭。
type Connection interface {
	// Send 发送数据到连接的另一端。
	// data 参数是要发送的字节数据。
	// 返回发送过程中遇到的错误，成功时返回 nil。
	Send(data []byte) error

	// Recv 从连接接收数据。
	// 返回接收到的字节数据和可能的错误。
	// 如果连接已关闭，应返回相应的错误。
	Recv() ([]byte, error)

	// Close 关闭连接并释放相关资源。
	// 返回关闭过程中遇到的错误，成功时返回 nil。
	Close() error

	// RemoteAddress 返回连接的远程地址。
	RemoteAddress() string

	// LocalAddress 返回连接的本地地址。
	LocalAddress() string
}

