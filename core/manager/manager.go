package manager

import (
	"fmt"
	"github.com/kercylan98/vivid/core"
	"github.com/puzpuzpuz/xsync/v3"
)

var _ core.Manager = (*manager)(nil)

type manager struct {
	options   core.ManagerOptionsFetcher
	host      core.Host                             // 主机地址
	root      core.Process                          // 根进程
	processes *xsync.MapOf[core.Path, core.Process] // 用于存储所有进程的映射表
}

func (mgr *manager) GetHost() core.Host {
	return mgr.host
}

func (mgr *manager) Run() (err error) {
	if err = mgr.initServer(); err != nil {
		return
	}

	return
}

func (mgr *manager) RegisterProcess(process core.Process) (id core.ID, exist bool) {
	processId := process.GetID()
	if processId == nil {
		panic(fmt.Errorf("process id is nil"))
	}
	process, exist = mgr.processes.LoadOrStore(processId.GetPath(), process)
	if !exist {
		return processId, exist
	}
	return process.GetID(), exist
}

func (mgr *manager) UnregisterProcess(operator, id core.ID) {
	if id == nil {
		panic(fmt.Errorf("process id is nil"))
	}
	p, exist := mgr.processes.LoadAndDelete(id.GetPath())
	if !exist {
		return
	}
	p.OnTerminate(operator)
}

// GetProcess 获取一个进程
func (mgr *manager) GetProcess(id core.ID) (process core.Process) {
	if id == nil {
		return mgr.root
	}

	processCache := id.GetProcessCache()
	if processCache != nil {
		if !processCache.Terminated() {
			return process
		}

		id.SetProcessCache(nil)
	}

	//if !rc.Belong(id) {
	//	// 远程进程加载
	//	for _, resolver := range rc.par {
	//		if core = resolver.Resolve(id); core != nil {
	//			var anyProcess any = core
	//			prcv1.StoreProcessIdCache(id, &anyProcess)
	//			return
	//		}
	//	}
	//	return rc.config.notFoundSubstitute
	//}

	// 本地进程加载
	var exist bool
	process, exist = mgr.processes.Load(id.GetPath())
	if exist {
		id.SetProcessCache(process)
		return process
	} else {
		return mgr.root
	}
}

func (mgr *manager) initServer() error {
	server := mgr.options.FetchServer()
	if server == nil {
		return nil
	}

	go func(server core.Server) {
		for envelope := range server.GetEnvelopeChannel() {
			mgr.GetProcess(envelope.ReceiverID()).Send(envelope)
		}
	}(server)

	return nil
}
