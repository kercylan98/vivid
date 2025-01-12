package process

import (
	"github.com/kercylan98/vivid/core"
	"github.com/kercylan98/vivid/core/id"
)

var (
	_builder core.ProcessBuilder = &builder{}
)

func Builder() core.ProcessBuilder {
	return _builder
}

type builder struct{}

func (i *builder) HostOf(host core.Host) core.Process {
	return &implOfProcess{
		id: id.Builder().Build(host, "/"),
	}
}
