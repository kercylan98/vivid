package remoting

type NetworkEnvelopHandler interface {
	HandleRemotingEnvelop(system bool, agentAddr, agentPath, senderAddr, senderPath, receiverAddr, receiverPath string, messageInstance any)
}
