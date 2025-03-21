package actor

import "github.com/kercylan98/vivid/src/vivid/internal/core"

// RelationContext 是 Actor 之间的关系上下文接口，其中定义了与 Actor 之间的关系相关的方法
type RelationContext interface {
	// NextGuid 返回下一个子 Actor 的 GUID 并使其计数器加一
	NextGuid() int64

	// BindChild 将指定的子 Actor 绑定到当前 Actor 上
	BindChild(child Ref)

	// UnbindChild 将指定的子 Actor 从当前 Actor 上解绑
	UnbindChild(child Ref)

	// Children 返回当前 Actor 的所有子 Actor
	Children() map[core.Path]Ref

	// Watch 监视指定 Actor 的生命周期结束信号
	Watch(ref Ref)

	// Unwatch 取消监视指定 Actor 的生命周期结束信号
	Unwatch(ref Ref)

	// AddWatcher 添加一个监视当前 Actor 的 Actor
	AddWatcher(ref Ref)

	// RemoveWatcher 移除一个监视当前 Actor 的 Actor
	RemoveWatcher(ref Ref)

	// Watchers 返回监视当前 Actor 的所有 Actor
	Watchers() map[core.URL]Ref

	// ResetWatchers 重置监视当前 Actor 的所有 Actor
	ResetWatchers()
}
