package id

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/resource"
	"sync/atomic"
)

func init() {
	vivid.GetMessageRegister().RegisterName("_vivid.core.id", new(id))
}

var _ vivid.ID = (*id)(nil)

type id struct {
	Host  resource.Host
	Path  resource.Path
	cache atomic.Pointer[vivid.Process]
}

func (id *id) GetPath() resource.Path {
	return id.Path
}

func (id *id) GetProcessCache() vivid.Process {
	cache := id.cache.Load()
	if cache == nil {
		return nil
	}
	return *cache
}

func (id *id) SetProcessCache(process vivid.Process) {
	if process == nil {
		id.cache.Store(nil)
	} else {
		id.cache.Store(&process)
	}
}
