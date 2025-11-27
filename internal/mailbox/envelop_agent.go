package mailbox

import "github.com/kercylan98/vivid"

type EnvelopAgent interface {
	Reply(message vivid.Message)
}
