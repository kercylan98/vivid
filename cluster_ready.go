package vivid

// ClusterJoinResult 表示本次启动的初始加入结果，不代表当前可达性或多数派授权。
type ClusterJoinResult struct {
	LocalID      string // 本次启动的实例 ID
	Bootstrapped bool   // true 表示独立启动，false 表示已收到种子的有效加入响应
	SeedAddress  string // 接受加入的种子广告地址，独立启动时为空
}
