package id

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/resource"
	"sync/atomic"
)

func init() {
	vivid2.GetMessageRegister().RegisterName("_vivid.core.id", new(id))
}

var _ vivid2.ID = (*id)(nil)

type id struct {
	Host  resource.Host
	Path  resource.Path
	cache atomic.Pointer[vivid2.Process]
}

func (id *id) GetPath() resource.Path {
	return id.Path
}

func (id *id) GetProcessCache() vivid2.Process {
	cache := id.cache.Load()
	if cache == nil {
		return nil
	}
	return *cache
}

func (id *id) SetProcessCache(process vivid2.Process) {
	if process == nil {
		id.cache.Store(nil)
	} else {
		id.cache.Store(&process)
	}
}
