package virtual

type (
	Kind = string // 虚拟 Actor 的种类，与 VirtualActorProviders 的 key 对应。
	Name = string // 虚拟 Actor 的实例名称，同一 Kind 下唯一。
)
