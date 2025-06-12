package processor

// RPCConn 定义了 RPC 连接的接口。
//
// RPCConn 抽象了网络连接的基本操作，用于 Actor 系统的远程通信。
// 它提供了发送、接收和关闭连接的基本功能。
//
// 实现者需要处理：
//   - 网络数据的序列化和反序列化
//   - 连接的生命周期管理
//   - 错误处理和重连逻辑
type RPCConn interface {
	// Send 发送数据到远程端点。
	//
	// 参数 bytes 是要发送的字节数据。
	// 返回发送过程中可能出现的错误。
	Send(bytes []byte) error

	// Recv 从远程端点接收数据。
	//
	// 此方法会阻塞直到接收到数据或发生错误。
	// 返回接收到的字节数据和可能的错误。
	Recv() ([]byte, error)

	// Close 关闭连接。
	//
	// 关闭连接并释放相关资源。
	// 返回关闭过程中可能出现的错误。
	Close() error
}

// RPCConnProvider 定义了 RPC 连接提供者接口。
//
// 用于创建到指定地址的 RPC 连接，支持连接池和连接复用。
type RPCConnProvider interface {
	// Provide 创建到指定地址的 RPC 连接。
	//
	// 参数 address 是目标地址，格式取决于具体的实现。
	// 返回创建的连接实例和可能的错误。
	Provide(address string) (RPCConn, error)
}

// RPCConnProviderFN 是 RPCConnProvider 接口的函数式实现。
//
// 允许使用函数直接实现 RPC 连接提供者。
type RPCConnProviderFN func(address string) (RPCConn, error)

// Provide 实现 RPCConnProvider 接口的 Provide 方法。
func (fn RPCConnProviderFN) Provide(address string) (RPCConn, error) {
	return fn(address)
}
