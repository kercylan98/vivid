package remoting

import (
	"sync"

	"github.com/kercylan98/vivid"
)

func newMailboxCentral(remotingServerRef vivid.ActorRef, actorLiaison vivid.ActorLiaison, codec vivid.Codec) *MailboxCentral {
	return &MailboxCentral{
		codec:             codec,
		actorLiaison:      actorLiaison,
		remotingServerRef: remotingServerRef,
		mailboxes:         make(map[string]*Mailbox),
	}
}

type MailboxCentral struct {
	codec             vivid.Codec         // 编解码器
	actorLiaison      vivid.ActorLiaison  // 演员联络员
	remotingServerRef vivid.ActorRef      // 远程服务器 ActorRef
	mailboxes         map[string]*Mailbox // 远程邮箱集合
	lock              sync.Mutex          // 锁
}

func (rmc *MailboxCentral) Close() {
	rmc.lock.Lock()
	defer rmc.lock.Unlock()

	for _, mailbox := range rmc.mailboxes {
		mailbox.connectionLock.Lock()
		for _, connection := range mailbox.connections {
			if connection != nil {
				connection.Close()
			}
		}
		mailbox.connectionLock.Unlock()
	}
}

func (rmc *MailboxCentral) GetOrCreate(advertiseAddr string, envelopHandler NetworkEnvelopHandler) *Mailbox {
	rmc.lock.Lock()
	defer rmc.lock.Unlock()

	m, ok := rmc.mailboxes[advertiseAddr]
	if !ok {
		m = newMailbox(advertiseAddr, rmc.codec, envelopHandler, rmc.actorLiaison, rmc.remotingServerRef)
		rmc.mailboxes[advertiseAddr] = m
	}

	return m
}
