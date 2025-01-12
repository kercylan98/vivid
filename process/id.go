package main

import "sync/atomic"

var (
	_ ID        = (*implOfID)(nil)
	_ IDBuilder = (*implOfIDBuilder)(nil)
)

var (
	idBuilder IDBuilder = &implOfIDBuilder{}
)

type (
	// Host 是一个主机地址，它是一个字符串，例如：localhost
	Host = string

	// Path 是一个路径，它是一个字符串，例如：/a/b/c
	Path = string
)

func NewIDBuilder() IDBuilder {
	return idBuilder
}

type IDBuilder interface {
	Build(host Host, path Path) (id ID)
}

type implOfIDBuilder struct{}

func (i *implOfIDBuilder) Build(host Host, path Path) (id ID) {
	return &implOfID{
		host: host,
		path: path,
	}
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

type implOfID struct {
	host  Host
	path  Path
	cache atomic.Pointer[Process]
}

func (id *implOfID) GetPath() Path {
	return id.path
}

func (id *implOfID) GetProcessCache() Process {
	cache := id.cache.Load()
	if cache == nil {
		return nil
	}
	return *cache
}

func (id *implOfID) SetProcessCache(process Process) {
	if process == nil {
		id.cache.Store(nil)
	} else {
		id.cache.Store(&process)
	}
}
