package system

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/wasteland/src/wasteland"
)

type Config struct {
	LoggerProvider log.Provider      // 日志提供者
	Address        wasteland.Address // 网络地址
}
