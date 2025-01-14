package vivid

import (
	"fmt"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	_ processManager = (*processManagerImpl)(nil) // 确保 processManagerImpl 实现了 processManager 接口
)

// newProcessManager 创建一个 processManagerImpl 实例，它是 processManager 的唯一实现
func newProcessManager(host Host) *processManagerImpl {
	return &processManagerImpl{
		host:      host,
		processes: xsync.NewMapOf[Path, Process](),
	}
}

// processManager 是内部对于 Process 的管理器的抽象，它管理了所有进程的生命周期，并以接口的形式确保可测试性
type processManager interface {

	// setDaemon 设置守护进程
	setDaemon(process Process)

	// registerProcess 注册一个进程到管理器中，当 id 为 nil 时将返回错误。
	//   - 如果进程已经存在则返回 true，并且返回已有进程的 ID
	//   - 如果进程不存在则返回 false，并且返回新进程的 ID
	registerProcess(process Process) (id ID, exist bool, err error)

	// unregisterProcess 注销一个进程，当 id 为 nil 时将导致 panic。
	//   - 如果进程不存在则不做任何操作
	//   - 如果进程存在则调用进程的 OnTerminate 方法
	unregisterProcess(operator, id ID)

	// getProcess 获取一个进程，当 id 为 nil 时返回守护进程。
	getProcess(id ID) (process Process, daemon bool)

	// getHost 获取主机地址
	getHost() Host
}

type processManagerImpl struct {
	daemon    Process                     // 守护进程
	host      Host                        // 主机地址
	processes *xsync.MapOf[Path, Process] // 用于存储所有进程的映射表
}

func (mgr *processManagerImpl) setDaemon(process Process) {
	mgr.daemon = process
}

func (mgr *processManagerImpl) registerProcess(process Process) (id ID, exist bool, err error) {
	processId := process.GetID()
	if processId == nil {
		return nil, false, fmt.Errorf("process id is nil")
	}
	process, exist = mgr.processes.LoadOrStore(processId.GetPath(), process)
	if !exist {
		return processId, exist, nil
	}
	return process.GetID(), exist, nil
}

func (mgr *processManagerImpl) unregisterProcess(operator, id ID) {
	if id == nil {
		return
	}
	p, exist := mgr.processes.LoadAndDelete(id.GetPath())
	if !exist {
		return
	}
	p.OnTerminate(operator)
}

func (mgr *processManagerImpl) getProcess(id ID) (process Process, daemon bool) {
	if id == nil {
		return mgr.daemon, true
	}

	processCache := id.GetProcessCache()
	if processCache != nil {
		if !processCache.Terminated() {
			return process, false
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
		return process, false
	} else {
		return mgr.daemon, true
	}
}

func (mgr *processManagerImpl) getHost() Host {
	return mgr.host
}
