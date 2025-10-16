package registry

// Transport 表示传输层的抽象接口。
// 该接口用于创建到远程地址的客户端连接。
type Transport interface {
	// Dial 建立到指定地址的连接。
	// address 参数是要连接的远程地址。
	// 返回建立的连接和可能的错误。
	Dial(address string) (Connection, error)
}

