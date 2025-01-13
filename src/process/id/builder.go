package id

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/resource"
)

var (
	_builder vivid.IDBuilder = &builder{}
)

func Builder() vivid.IDBuilder {
	return _builder
}

type builder struct{}

func (i *builder) Build(host resource.Host, path resource.Path) vivid.ID {
	return &id{
		Host: host,
		Path: path,
	}
}
