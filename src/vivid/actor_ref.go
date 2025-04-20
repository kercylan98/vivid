package vivid

// ActorRef 是 Actor 的引用接口，用于向 Actor 发送消息，
// 它隐藏了 Actor 的实现细节，提供了位置透明性，
// ActorRef 是不可变的，可以安全地在不同的线程之间传递，
// 通过 ActorRef 可以获取 Actor 的地址、路径等信息，但不能直接访问 Actor 的状态。
type ActorRef interface {
	// Address 获取 Actor 的网络地址，包含主机名和端口号。
	Address() Address

	// Path 获取 Actor 的路径。
	//
	// 路径是 Actor 在 Actor 系统中的唯一标识。
	Path() Path

	// String 返回 Actor 引用的字符串表示，
	// 这通常是 Actor 路径的字符串形式。
	String() string

	// URL 返回 Actor 的 URL 表示。
	URL() string
}
