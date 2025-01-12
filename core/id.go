package core

type IDBuilder interface {
	Build(host Host, path Path) (id ID)
}

// ID 是对于一个进程的唯一标识，它是一个可以跨网络传输的数据结构
type ID interface {
	// GetPath 返回这个 ID 的路径
	GetPath() Path

	// GetProcessCache 返回这个 ID 的进程缓存
	GetProcessCache() Process

	// SetProcessCache 设置这个 ID 的进程缓存
	SetProcessCache(process Process)
}
