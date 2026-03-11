package remoting

import "github.com/kercylan98/vivid"

type NetworkEnvelopHandler interface {
	HandleRemotingEnvelop(system bool, sender, receiver string, messageInstance any) error

	HandleFailedRemotingEnvelop(envelop vivid.Envelop)
}
