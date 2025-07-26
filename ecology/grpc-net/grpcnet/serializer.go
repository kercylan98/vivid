package grpcnet

import (
	"fmt"
	"github.com/kercylan98/vivid/pkg/provider"
	serializerIntercace "github.com/kercylan98/vivid/pkg/serializer"
	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func newSerializerProvider() provider.FN[serializerIntercace.NameSerializer] {
	return func() serializerIntercace.NameSerializer {
		return new(serializer)
	}
}

type serializer struct{}

func (s *serializer) Serialize(data any) (typeName string, serializedData []byte, err error) {
	protoMessage, ok := data.(proto.Message)
	if !ok {
		return "", nil, fmt.Errorf("not support message type %T", data)
	}

	typeName = string(protoMessage.ProtoReflect().Type().Descriptor().FullName())
	serializedData, err = proto.Marshal(protoMessage)
	return
}

func (s *serializer) Deserialize(typeName string, serializedData []byte) (result any, err error) {
	messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(typeName))
	if err != nil {
		return nil, err
	}

	m := messageType.New().Interface()
	if err = proto.Unmarshal(serializedData, m); err != nil {
		return nil, err
	}

	return m, nil
}
