package cluster

const (
	// maxGetNodesResponseMembers 反序列化 GetNodesResponse 时允许的 Members 数量上限，防止恶意或异常消息导致 OOM。
	maxGetNodesResponseMembers = 65536
)
