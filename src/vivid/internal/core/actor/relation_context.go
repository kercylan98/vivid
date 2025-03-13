package actor

// RelationContext 是 Actor 之间的关系上下文接口，其中定义了与 Actor 之间的关系相关的方法
type RelationContext interface {
	// NextGuid 返回下一个子 Actor 的 GUID 并使其计数器加一
	NextGuid() int64

	// BindChild 将指定的子 Actor 绑定到当前 Actor 上
	BindChild(child Ref)

	// UnbindChild 将指定的子 Actor 从当前 Actor 上解绑
	UnbindChild(child Ref)
}
