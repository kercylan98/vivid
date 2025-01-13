package id

import (
	"github.com/kercylan98/vivid/core"
	"sync/atomic"
)

func init() {
	core.GetMessageRegister().Register(new(id))
}

var _ core.ID = (*id)(nil)

type id struct {
	Host  core.Host
	Path  core.Path
	cache atomic.Pointer[core.Process]
}

func (id *id) GetPath() core.Path {
	return id.Path
}

func (id *id) GetProcessCache() core.Process {
	cache := id.cache.Load()
	if cache == nil {
		return nil
	}
	return *cache
}

func (id *id) SetProcessCache(process core.Process) {
	if process == nil {
		id.cache.Store(nil)
	} else {
		id.cache.Store(&process)
	}
}
