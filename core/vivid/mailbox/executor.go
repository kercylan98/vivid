package mailbox

type Executor interface {
	OnSystemMessage(message any)

	OnUserMessage(message any)
}
