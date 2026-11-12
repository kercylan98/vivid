package vivid

import "github.com/kercylan98/vivid/internal/serialization"

type (
	// Codec 定义外部消息体的编解码逻辑，消息类型标识由框架处理。
	Codec        = serialization.Codec
	Reader       = serialization.Reader
	Writer       = serialization.Writer
	DecodeLimits = serialization.DecodeLimits
)

// ErrDecodeLimit 表示解码超出资源预算，可通过 errors.Is 判断。
var ErrDecodeLimit = serialization.ErrDecodeLimit

// DefaultDecodeLimits 返回默认解码预算。
func DefaultDecodeLimits() DecodeLimits { return serialization.DefaultDecodeLimits() }

// NewReader 创建独立的字段读取器，可选预算以最后一份为准，读取期间输入数据应保持不变。
func NewReader(data []byte, limits ...DecodeLimits) *Reader {
	return serialization.NewReader(data, limits...)
}

// NewWriter 创建独立的字段写入器。
func NewWriter() *Writer { return serialization.NewWriter() }
