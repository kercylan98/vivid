package main

import (
	"fmt"
	"github.com/puzpuzpuz/xsync/v3"
	"net"
)

var (
	_ Manager        = (*implOfManager)(nil)
	_ ManagerBuilder = (*implOfManagerBuilder)(nil)
)

var (
	managerBuilder ManagerBuilder = &implOfManagerBuilder{}
)

func NewManagerBuilder() ManagerBuilder {
	return managerBuilder
}

type ManagerBuilder interface {
	Build() (manager Manager)

	HostOf(host string) (manager Manager, err error)

	ListenerOf(listener net.Listener) (manager Manager, err error)
}

type implOfManagerBuilder struct{}

func (i *implOfManagerBuilder) Build() (manager Manager) {
	mgr := &implOfManager{
		host:      "localhost",
		processes: xsync.NewMapOf[Path, Process](),
	}
	mgr.root = NewProcessBuilder().HostOf(mgr.host)
	return mgr
}

func (i *implOfManagerBuilder) HostOf(host Host) (manager Manager, err error) {
	mgr := i.Build().(*implOfManager)
	mgr.host = host
	return mgr, nil
}

func (i *implOfManagerBuilder) ListenerOf(listener net.Listener) (manager Manager, err error) {
	panic("implement me")
}

type Manager interface {
	GetHost() Host

	RegisterProcess(process Process) (id ID, exist bool)

	UnregisterProcess(operator, id ID)

	GetProcess(id ID) Process
}

type implOfManager struct {
	host      Host                        // 主机地址
	root      Process                     // 根进程
	processes *xsync.MapOf[Path, Process] // 用于存储所有进程的映射表
}

func (mgr *implOfManager) GetHost() Host {
	return mgr.host
}

func (mgr *implOfManager) init() {

}

func (mgr *implOfManager) RegisterProcess(process Process) (id ID, exist bool) {
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

func (mgr *implOfManager) UnregisterProcess(operator, id ID) {
	if id == nil {
		panic(fmt.Errorf("process id is nil"))
	}
	process, exist := mgr.processes.LoadAndDelete(id.GetPath())
	if !exist {
		return
	}
	process.OnTerminate(operator)
}

// GetProcess 获取一个进程
func (mgr *implOfManager) GetProcess(id ID) (process Process) {
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
	//		if process = resolver.Resolve(id); process != nil {
	//			var anyProcess any = process
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
