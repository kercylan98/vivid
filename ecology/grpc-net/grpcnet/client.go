package grpcnet

import (
	"context"
	"github.com/kercylan98/vivid/grpc-net/grpcnet/internal/stream"
	"github.com/kercylan98/vivid/pkg/vivid/processor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newConnectorProvider() processor.RPCConnProvider {
	return processor.RPCConnProviderFN(func(address string) (processor.RPCConn, error) {
		cc, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}

		streamClient := stream.NewStreamClient(cc)
		streaming, err := streamClient.ActorStreaming(context.Background())
		if err != nil {
			return nil, err
		}
		return newClientConn(streaming), nil
	})
}
