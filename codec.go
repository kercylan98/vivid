package vivid

import "github.com/kercylan98/vivid/internal/serialization"

type (
	// Codec 定义外部消息体的编解码逻辑，消息类型标识由框架处理。
	Codec  = serialization.Codec
	Reader = serialization.Reader
	Writer = serialization.Writer
)

// NewReader 创建独立的字段读取器，读取期间输入数据应保持不变。
func NewReader(data []byte) *Reader { return serialization.NewReader(data) }

// NewWriter 创建独立的字段写入器。
func NewWriter() *Writer { return serialization.NewWriter() }
