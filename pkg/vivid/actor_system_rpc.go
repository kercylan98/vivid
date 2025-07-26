package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/serializer"
	processorOutside "github.com/kercylan98/vivid/pkg/vivid/processor"
)

type actorSystemRPCSerializer struct {
	outside serializer.NameSerializer
}

func (a *actorSystemRPCSerializer) Serialize(data any) (typeName string, serializedData []byte, err error) {
	if internal, ok := data.(internalMessage); ok {
		typeName, serializedData = internal.marshal()
		return
	}
	return a.outside.Serialize(data)
}

func (a *actorSystemRPCSerializer) Deserialize(typeName string, serializedData []byte) (result any, err error) {
	im := provideInternalMessageInstance(typeName)
	if im == nil {
		return a.outside.Deserialize(typeName, serializedData)
	}
	im.unmarshal(serializedData)
	return im, nil
}

type actorSystemRPC struct {
	actorSystem *actorSystem
}

func (sys *actorSystemRPC) OnMessage(conn processorOutside.RPCConn, serializer serializer.NameSerializer, data []byte) {
	var batchMessage = processorOutside.NewRPCBatchMessage()
	if err := batchMessage.Unmarshal(data); err != nil {
		sys.actorSystem.Logger().Error("batchMessage.unmarshal", log.Err(err))
		if err = conn.Close(); err != nil {
			sys.actorSystem.Logger().Error("conn.Close", log.Err(err))
		}
		return
	}

	for i := range batchMessage.Len() {
		sender, target, name, raw, system := batchMessage.Get(i)
		message, err := serializer.Deserialize(name, raw)
		if err != nil {
			sys.actorSystem.Logger().Warn("rpc.message", log.Err(err))
		}

		senderRef, targetRef := ParseActorRef(sender), ParseActorRef(target)
		if senderRef == nil {
			sys.actorSystem.tell(senderRef, targetRef, message, system)
		} else {
			sys.actorSystem.probe(senderRef, targetRef, message, system)
		}
	}
}
