package grpcnet

import "github.com/kercylan98/vivid/pkg/vivid"

func NewNetworkConfiguration(bindAddr string, advertisedAddr ...string) *vivid.ActorSystemNetworkConfiguration {
	return vivid.NewActorSystemNetworkConfiguration(
		vivid.WithActorSystemNetworkConfigurationNetwork("tcp"),
		vivid.WithActorSystemNetworkConfigurationBindAddress(bindAddr),
		vivid.WithActorSystemNetworkConfigurationAdvertisedAddress(func() string {
			if len(advertisedAddr) > 0 {
				return advertisedAddr[0]
			}
			return bindAddr
		}()),
		vivid.WithActorSystemNetworkConfigurationServer(newServer()),
		vivid.WithActorSystemNetworkConfigurationConnector(newConnectorProvider()),
		vivid.WithActorSystemNetworkConfigurationSerializerProvider(newSerializerProvider()),
	)
}
