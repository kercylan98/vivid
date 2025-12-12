package mailbox

import (
	"sync"
)

// TODO: 远程邮箱的客户端应当以 Actor 方式实现，复用 ConnectionActor；
func NewRemotingMailboxCentral() *RemotingMailboxCentral {
	return &RemotingMailboxCentral{
		mailboxes: make(map[string]*RemotingMailbox),
	}
}

type RemotingMailboxCentral struct {
	mailboxes map[string]*RemotingMailbox
	lock      sync.Mutex
}

func (rmc *RemotingMailboxCentral) GetOrCreate(advertiseAddr string) *RemotingMailbox {
	rmc.lock.Lock()
	defer rmc.lock.Unlock()

	m, ok := rmc.mailboxes[advertiseAddr]
	if !ok {
		m = NewRemotingMailbox(advertiseAddr)
	}

	return m
}
