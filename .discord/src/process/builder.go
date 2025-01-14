package process

import (
	"github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/process/id"
	"github.com/kercylan98/vivid/.discord/src/resource"
)

var (
	_builder vivid.ProcessBuilder = &builder{}
)

func Builder() vivid.ProcessBuilder {
	return _builder
}

type builder struct{}

func (i *builder) HostOf(host resource.Host) vivid.Process {
	return &implOfProcess{
		id: id.Builder().Build(host, "/"),
	}
}
