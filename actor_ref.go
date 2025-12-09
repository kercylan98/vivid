package vivid

import "net"

type ActorRef interface {
	GetAddress() net.Addr
	GetPath() ActorPath
}
