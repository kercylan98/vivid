package remoting

import (
	"crypto/tls"
	"net"
)

func newListener(bindAddr string, tlsConfig *tls.Config) (l *listener, err error) {
	l = new(listener)
	if tlsConfig != nil {
		l.Listener, err = tls.Listen("tcp", bindAddr, tlsConfig)
	} else {
		l.Listener, err = net.Listen("tcp", bindAddr)
	}
	return l, err
}

type listener struct {
	net.Listener
}
