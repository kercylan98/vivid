package vivid

import (
	"maps"
	"time"
	"unicode/utf8"

	"github.com/kercylan98/vivid/internal/utils"
)

// ActorSystemCoordinationOptions 配置本系统托管的固定成员协调节点，与 Gossip 成员视图独立。
// NodeID 及 Members 的编号必须跨重启保持稳定；每个成员使用独立系统和 WAL。
type ActorSystemCoordinationOptions struct {
	GroupID           string            // 协调组标识，必填，最多 128 字节
	NodeID            uint64            // 本节点稳定编号，非零，必须存在于 Members
	Members           map[uint64]string // 固定投票成员的广告地址，1 至 256 个，每个地址唯一
	Path              string            // 本节点 WAL 路径，父目录须已存在
	Create            bool              // true 仅用于首次创建；false 恢复已有数据，缺失时报错
	HeartbeatInterval time.Duration     // 心跳与驱动周期，0 使用 100ms
	ElectionMin       time.Duration     // 最短竞选等待，0 使用 1s，至少为心跳周期的三倍
	ElectionMax       time.Duration     // 最长竞选等待，0 使用 2s，须大于 ElectionMin
}

func cloneCoordinationOptions(options *ActorSystemCoordinationOptions) *ActorSystemCoordinationOptions {
	if options == nil {
		return nil
	}
	copy := *options
	copy.Members = maps.Clone(options.Members)
	if copy.HeartbeatInterval == 0 {
		copy.HeartbeatInterval = 100 * time.Millisecond
	}
	if copy.ElectionMin == 0 {
		copy.ElectionMin = time.Second
	}
	if copy.ElectionMax == 0 {
		copy.ElectionMax = 2 * time.Second
	}
	return &copy
}

// Validate 校验固定成员和时序，不修改配置、不解析 DNS，也不创建存储。
func (o *ActorSystemCoordinationOptions) Validate() error {
	if o == nil {
		return ErrorIllegalArgument.WithMessage("coordination options are nil")
	}
	n := cloneCoordinationOptions(o)
	if n.GroupID == "" || len(n.GroupID) > 128 || !utf8.ValidString(n.GroupID) || n.NodeID == 0 || len(n.Members) == 0 || len(n.Members) > 256 || n.Path == "" {
		return ErrorIllegalArgument.WithMessage("coordination requires a group, node ID, fixed members and WAL path")
	}
	if n.HeartbeatInterval <= 0 || n.ElectionMin <= 0 || n.HeartbeatInterval > n.ElectionMin/3 || n.ElectionMax <= n.ElectionMin {
		return ErrorIllegalArgument.WithMessage("coordination requires positive intervals, election min at least three heartbeats, and election max greater than min")
	}
	addresses := make(map[string]bool, len(n.Members))
	for id, address := range n.Members {
		normalized, ok := utils.NormalizeRemoteAddress(address)
		if id == 0 || !ok || addresses[normalized] {
			return ErrorIllegalArgument.WithMessage("coordination members require nonzero IDs and unique remote addresses")
		}
		addresses[normalized] = true
	}
	if _, exists := n.Members[n.NodeID]; !exists {
		return ErrorIllegalArgument.WithMessage("coordination node ID is absent from members")
	}
	return nil
}

// WithActorSystemCoordination 托管本地协调节点，复制配置；需同时启用 Remoting。
// Start 确认本地 WAL 初始化成功，选主异步进行，不表示已经取得多数派。
func WithActorSystemCoordination(options ActorSystemCoordinationOptions) ActorSystemOption {
	snapshot := cloneCoordinationOptions(&options)
	return func(o *ActorSystemOptions) { o.CoordinationOptions = cloneCoordinationOptions(snapshot) }
}

type CoordinationRole uint8

const (
	CoordinationFollower CoordinationRole = iota
	CoordinationCandidate
	CoordinationLeader
)

// CoordinationStatus 是本地协调节点的观察，不是实时健康、多数派授权或写入租约。
type CoordinationStatus struct {
	GroupID string
	NodeID  uint64
	Role    CoordinationRole
	Term    uint64
	Leader  uint64 // 本地所知 Leader，0 表示未知
	Commit  uint64 // 本地提交位置
	Applied uint64 // 本地状态机应用位置
}
