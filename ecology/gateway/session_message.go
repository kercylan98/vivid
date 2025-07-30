package gateway

import (
	"github.com/kercylan98/vivid/pkg/provider"
	"time"
)

type CodecProvider = provider.Provider[Codec]
type CodecProviderFN = provider.FN[Codec]

type Codec interface {
	Encoder
	Decoder
}

type Decoder interface {
	Decode(bytes []byte) (SessionC2SMessage, error)
}

type Encoder interface {
	Encode(message SessionS2CMessage) ([]byte, error)
}

type SessionC2SMessage interface {
	GetType() int32
	GetProtocolId() int32
	GetC2SSendTime() time.Time
	GetPayload() []byte
}

type SessionS2CMessage interface {
	SessionC2SMessage
	GetRecvTime() time.Time
	GetS2CTime() time.Time
}
