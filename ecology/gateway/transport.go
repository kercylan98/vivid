package gateway

type Transport interface {
	// GetSessionId 获取传输层协议会话唯一标识。
	GetSessionId() string

	// GetSessionName 获取传输层协议会话可重复名称。
	GetSessionName() string

	// Read 通过传输层协议读取数据。
	Read() (data []byte, err error)

	// Write 通过传输层协议写入数据。
	Write(data []byte) error

	// Close 关闭传输层。
	Close() error
}
