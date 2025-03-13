package actor

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
)

type Config struct {
	Name           string             // Actor 名称
	LoggerProvider log.Provider       // 日志提供者
	Mailbox        mailbox.Mailbox    // 邮箱
	Dispatcher     mailbox.Dispatcher // 邮箱调度器
}
