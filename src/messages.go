package vivid

var (
	onSuspendMailbox = new(onSuspendMailboxMessage)
	onResumeMailbox  = new(onResumeMailboxMessage)
)

type (

	// onSuspendMailboxMessage 暂停邮箱
	onSuspendMailboxMessage int8

	// onResumeMailboxMessage 恢复邮箱
	onResumeMailboxMessage int8
)
