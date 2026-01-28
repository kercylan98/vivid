package remoting

import (
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

func newMailboxCentral(remotingServerRef vivid.ActorRef, actorLiaison vivid.ActorLiaison, codec vivid.Codec, eventStream vivid.EventStream, options vivid.ActorSystemRemotingOptions) *MailboxCentral {
	return &MailboxCentral{
		codec:             codec,
		actorLiaison:      actorLiaison,
		remotingServerRef: remotingServerRef,
		eventStream:       eventStream,
		mailboxes:         make(map[string]*Mailbox),
		options:           options,
	}
}

type MailboxCentral struct {
	options           vivid.ActorSystemRemotingOptions
	codec             vivid.Codec         // 编解码器
	actorLiaison      vivid.ActorLiaison  // 演员联络员
	remotingServerRef vivid.ActorRef      // 远程服务器 ActorRef
	eventStream       vivid.EventStream   // 事件流
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
				if err := connection.Close(); err != nil {
					rmc.actorLiaison.Logger().Warn("close Remoting connection failed", log.String("advertise_addr", connection.advertiseAddr), log.Any("err", err))
				}
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
		m = newMailbox(advertiseAddr, rmc.codec, envelopHandler, rmc.actorLiaison, rmc.remotingServerRef, rmc.eventStream, rmc.options)
		rmc.mailboxes[advertiseAddr] = m
	}

	return m
}
