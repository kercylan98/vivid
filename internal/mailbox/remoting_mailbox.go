package mailbox

import (
	"net"

	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Mailbox = &RemotingMailbox{}
)

func NewRemotingMailbox(sender vivid.ActorRef, advertiseAddress net.Addr) *RemotingMailbox {
	return &RemotingMailbox{
		sender:           sender,
		advertiseAddress: advertiseAddress,
	}
}

type RemotingMailbox struct {
	sender           vivid.ActorRef
	advertiseAddress net.Addr
}

func (m *RemotingMailbox) Enqueue(envelop vivid.Envelop) {
	// TODO: 发送消息到远程 Actor
	panic("not implemented: send message to remote actor")
}
