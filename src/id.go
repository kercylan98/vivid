package vivid

import (
	"hash/fnv"
	"net/url"
	"strings"
	"sync/atomic"
)

var (
	_ ID = (*defaultID)(nil) // 确保 defaultID 实现了 ID 接口
)

// ActorRef 是 ID 的别名，用于表示一个 Actor 的唯一标识
type ActorRef = ID

// ID 是一个可以跨网络传输的唯一标识的抽象。为了支持通过 Protocol Buffers 或其他方式进行序列化，ID 在 Vivid 中被定义为接口。
type ID interface {
	// GetHost 返回这个 ID 所属的主机地址
	GetHost() Host

	// GetPath 返回这个 ID 的资源路径
	GetPath() Path

	// GetProcessCache 返回这个 ID 的进程缓存
	GetProcessCache() Process

	// SetProcessCache 设置这个 ID 的进程缓存
	//   - 进程缓存是被用于加速进程查找的功能，当进程可用时，可通过进程缓存直接获取进程，而不需要通过进程管理器进行查找。
	SetProcessCache(process Process)

	// Clone 返回这个 ID 的一个副本
	Clone() ID

	// Sub 基于当前 ID 构建一个子 ID
	Sub(path Path) ID

	// String 返回这个 ID 的字符串表示
	String() string

	// Equal 判断两个 ID 是否相等
	Equal(id ID) bool

	// Hash 返回这个 ID 的哈希值
	Hash() uint32
}

// IDBuilder 是一个用于构建 ID 的接口。
//   - 由于 ID 可能存在多种构建方式，因此 IDBuilder 是一个接口，而不是一个具体的类型。
//
// 在使用 IDBuilder 时，应该是在构建之初便确定的，因此不应为此提供 Provider，避免在运行时动态更改 IDBuilder 而导致序列化方式不一致。
type IDBuilder interface {
	// BuildID 通过指定的主机地址和资源路径构建一个 ID
	BuildID(host Host, path Path) ID

	// BuildRootID 通过指定的主机地址构建一个根 ID
	BuildRootID(host Host) ID
}

type defaultIDBuilder struct{}

func (b *defaultIDBuilder) BuildID(host Host, path Path) ID {
	return &defaultID{
		Host: host,
		Path: path,
	}
}

func (b *defaultIDBuilder) BuildRootID(host Host) ID {
	return b.BuildID(host, "/")
}

// defaultID 是 ID 的默认实现，可以通过 GetIdBuilder
//   - 它满足基于 gob、json 的序列化和反序列化。
type defaultID struct {
	Host Host `json:"host,omitempty"` // 主机地址
	Path Path `json:"path,omitempty"` // 资源路径

	processCache atomic.Pointer[Process] // 进程缓存，该字段并非序列化的一部分
}

func (id *defaultID) Equal(other ID) bool {
	return id.GetHost() == other.GetHost() && id.GetPath() == other.GetPath()
}

func (id *defaultID) Sub(path Path) ID {
	if id.Path == "/" {
		return &defaultID{
			Host: id.Host,
			Path: "/" + path,
		}
	}
	return &defaultID{
		Host: id.Host,
		Path: strings.TrimRight(id.Path+"/"+path, "/"),
	}
}

func (id *defaultID) String() string {
	u := url.URL{
		Scheme: "vivid",
		Host:   id.Host,
		Path:   id.Path,
	}
	return u.String()
}

func (id *defaultID) Clone() ID {
	return &defaultID{
		Host: id.Host,
		Path: id.Path,
	}
}

func (id *defaultID) GetHost() Host {
	return id.Host
}

func (id *defaultID) GetPath() Path {
	return id.Path
}

func (id *defaultID) GetProcessCache() Process {
	cache := id.processCache.Load()
	if cache == nil {
		return nil
	}
	return *cache
}

func (id *defaultID) SetProcessCache(process Process) {
	id.processCache.Store(&process)
}

func (id *defaultID) Hash() uint32 {
	h := fnv.New32a()
	_, err := h.Write([]byte(id.Host))
	if err != nil {
		return 0
	}
	return h.Sum32()
}
