package remoting

import "github.com/kercylan98/vivid"

type NetworkEnvelopHandler interface {
	HandleRemotingEnvelop(system bool, agentAddr, agentPath, senderAddr, senderPath, receiverAddr, receiverPath string, messageInstance any) error

	HandleFailedRemotingEnvelop(envelop vivid.Envelop)
}
