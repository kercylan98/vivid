package gossip

import (
	"strings"

	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
)

// ClusterView 本节点维护的集群视图：成员列表与版本向量，用于因果合并与 peer 选择。
type ClusterView struct {
	members *memberlist.MemberList   // 已知节点信息，key 为节点 ID（ActorRef.String()）
	version *versionvector.VersionVector // 节点 ID -> 版本计数，用于判断视图新旧与合并
}

// NewClusterView 创建空视图（空成员列表与空版本向量）。
func NewClusterView(logger log.Logger) *ClusterView {
	return &ClusterView{
		members: memberlist.New(logger),
		version: versionvector.New(),
	}
}

// Members 返回成员列表，供调用方进行 Add/Upsert/Merge/Unseens 等操作。
func (v *ClusterView) Members() *memberlist.MemberList { return v.members }

// Version 返回版本向量，供调用方进行 Increment/IsBefore/Merge 等操作。
func (v *ClusterView) Version() *versionvector.VersionVector { return v.version }

// Fingerprint 返回当前视图的确定性指纹（版本向量 + 成员列表），用于收敛检测。
func (v *ClusterView) Fingerprint() string {
	if v == nil {
		return ""
	}
	return strings.Join([]string{v.version.Fingerprint(), v.members.Fingerprint()}, "|")
}
