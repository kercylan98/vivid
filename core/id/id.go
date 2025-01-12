package id

import (
	"github.com/kercylan98/vivid/core"
	"sync/atomic"
)

var _ core.ID = (*id)(nil)

type id struct {
	host  core.Host
	path  core.Path
	cache atomic.Pointer[core.Process]
}

func (id *id) GetPath() core.Path {
	return id.path
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
