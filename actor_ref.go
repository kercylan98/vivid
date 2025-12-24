package vivid

// ActorRef 定义了 Actor 的抽象引用类型，作为唯一标识和操作 Actor 实例的基本句柄。
//
// ActorRef 主要用于分布式和本地环境下消息投递、身份判断、路径定位及创建 Actor 引用拷贝等场景。
//
// 实现说明：
//   - 框架保证每个 ActorRef 实例均与唯一的 Actor 绑定，引用等价性由 Equals 方法保证。
//   - ActorRef 是线程安全的，推荐在多协程环境下广泛传递与复用。
//   - 不同实现可代表本地、本地代理或远程 Actor，调用方无需关心具体实现细节。
type ActorRef interface {
	// GetAddress 返回该 Actor 所属系统或节点的地址（例如 IP:端口、集群节点标识等）。
	//
	// 主要用于分布式场景下区分不同主机或节点上的 Actor 所在位置。
	GetAddress() string

	// GetPath 返回该 Actor 在所属系统下的唯一路径 ActorPath。
	//
	// 路径由父子关系与命名组成，用于唯一定位同一节点或系统内的 Actor 层级。
	GetPath() ActorPath

	// Equals 判断当前 ActorRef 是否与另一个 ActorRef 语义等价。
	//
	// 等价性依据实现，通常比较地址与路径，便于做集合去重、哈希表索引等。
	// 若比较对象为 nil 或类型不同，通常返回 false。
	Equals(other ActorRef) bool

	// Clone 返回当前 ActorRef 的独立副本，内容等价但实例化为新对象。
	//
	// 用于缓存、并发传递等需要 ActorRef 不受外部影响的场景。
	Clone() ActorRef
}
