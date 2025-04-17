package system

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/wasteland/src/wasteland"
)

type Config struct {
	LoggerProvider           log.Provider                // 日志提供者
	Address                  wasteland.Address           // 网络地址
	CodecProvider            wasteland.CodecProvider     // 编解码器提供者
	RPCMessageBuilder        wasteland.RPCMessageBuilder // RPC 消息构建器
	GuardDefaultRestartLimit int                         // 默认重启次数限制
}
