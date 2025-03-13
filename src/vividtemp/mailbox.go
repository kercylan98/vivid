package vividtemp

var _ Mailbox = (*mailboxImpl)(nil)

type Mailbox interface {
	MailboxHandler
	Initialize(dispatcher Dispatcher, handler MailboxHandler)
	Suspend()
	Resume()
}

type MailboxHandler interface {
	HandleSystemMessage(message Message)
	HandleUserMessage(message Message)
}
