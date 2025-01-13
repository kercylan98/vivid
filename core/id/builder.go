package id

import "github.com/kercylan98/vivid/core"

var (
	_builder core.IDBuilder = &builder{}
)

func Builder() core.IDBuilder {
	return _builder
}

type builder struct{}

func (i *builder) Build(host core.Host, path core.Path) core.ID {
	return &id{
		Host: host,
		Path: path,
	}
}
