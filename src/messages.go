package vivid

func init() {
	RegisterMessageName("vivid.OnLaunch", onLaunch)
}

var (
	onLaunch         = new(OnLaunch)
	onSuspendMailbox = new(onSuspendMailboxMessage)
	onResumeMailbox  = new(onResumeMailboxMessage)
)

type (
	// OnLaunch 在 Actor 启动时，将会作为第一条消息被处理，适用于初始化 Actor 状态等场景。
	OnLaunch int8

	onSuspendMailboxMessage int8

	onResumeMailboxMessage int8
)
