package server

import (
	"github.com/kercylan98/vivid/pkg/vivid"
)

var (
	_ vivid.ServerOptions        = (*options)(nil)
	_ vivid.ServerOptionsFetcher = (*options)(nil)
)

func Options() vivid.ServerOptions {
	return &options{
		connChannelSize: 1024,
	}
}

type options struct {
	connChannelSize int
}

func (o *options) WithConnChannelSize(size int) vivid.ServerOptions {
	o.connChannelSize = size
	return o
}

func (o *options) GetConnChannelSize() int {
	return o.connChannelSize
}
