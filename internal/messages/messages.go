package messages

import (
	"net"
)

var (
	internalMessageFactory = make(map[uint32]InternalMessageFactory)
	actorRefFactory        ActorRefFactory
)

func SetActorRefFactory(factory ActorRefFactory) {
	actorRefFactory = factory
}

const (
	RefMessageType uint32 = iota + 1
	OnLaunchMessageType
	OnKillMessageType
	OnKilledMessageType
)

type InternalMessageFactory = func() any
type InternalMessageReader = func(actorRefFactory ActorRefFactory, message any, reader *Reader) error
type InternalMessageWriter = func(actorRefFactory ActorRefFactory, message any, writer *Writer) error
type ActorRefFactory = func(address net.Addr, path string) any // 用于创建 ActorRef 的工厂函数

func RegisterInternalMessage(messageType uint32, factory InternalMessageFactory, reader InternalMessageReader, writer InternalMessageWriter) {
	internalMessageFactory[messageType] = factory
}

func NewInternalMessage(messageType uint32) any {
	return internalMessageFactory[messageType]()
}
