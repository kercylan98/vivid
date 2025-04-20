package vivid

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	"github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
)

// 这个文件包含了 Actor 模型中使用的核心类型的别名
// 这些别名使得用户可以直接使用这些类型，而不需要导入内部包

type (
	// Address 是 Actor 的网络地址类型，
	// 它包含了主机名和端口号，用于在分布式系统中定位 Actor。
	Address = core.Address

	// Path 是 Actor 在 Actor 系统中的路径，
	// 它是 Actor 的唯一标识，用于在 Actor 系统中定位 Actor。
	Path = core.Path

	// Message 是 Actor 之间传递的消息类型，
	// 所有发送给 Actor 的消息都必须实现这个接口。
	Message = core.Message

	// Future 是表示异步操作结果的类型，
	// 它用于在 Ask 模式中获取 Actor 的响应。
	Future = future.Future

	// Mailbox 是 Actor 的邮箱接口，
	// 它负责存储发送给 Actor 的消息，并按照一定的策略将消息分发给 Actor 处理。
	Mailbox = mailbox.Mailbox

	// Dispatcher 是消息调度器接口，
	// 它负责将消息从邮箱分发给 Actor 处理，并管理 Actor 的执行。
	Dispatcher = mailbox.Dispatcher
)
