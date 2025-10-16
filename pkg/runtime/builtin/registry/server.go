package registry

// Server 表示一个连接接受器的抽象接口。
// 该接口只负责提供连接，不管理服务器的生命周期。
// 这允许外部与现有的 HTTP、GRPC 等服务器集成。
type Server interface {
	// Accept 接受一个新的连接。
	// 该方法会阻塞直到有新连接到达或服务器关闭。
	// 返回新建立的连接和可能的错误。
	// 当服务器关闭时，应返回相应的错误。
	Accept() (Connection, error)
}
