package vivid

type MailboxQueue interface {
	Push(m any)

	Pop() any
}
