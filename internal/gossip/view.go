package gossip

import (
	"strings"

	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
)

// ClusterView 本节点维护的集群视图：实例成员列表与版本，用于同步和联络目标选择。
type ClusterView struct {
	members *memberlist.MemberList       // 已知节点信息，key 为实例 ID
	version *versionvector.VersionVector // 实例 ID -> 版本计数，用于判断成员记录新旧
}

// NewClusterView 创建空视图（空成员列表与空版本向量）。
func NewClusterView(logger log.Logger) *ClusterView {
	return &ClusterView{
		members: memberlist.New(logger),
		version: versionvector.New(),
	}
}

// Members 返回由 Actor 消息线程维护的实例成员列表。
func (v *ClusterView) Members() *memberlist.MemberList { return v.members }

// Version 返回由 Actor 消息线程维护的实例版本。
func (v *ClusterView) Version() *versionvector.VersionVector { return v.version }

// Fingerprint 返回当前视图的确定性指纹（版本向量 + 成员列表），用于收敛检测。
func (v *ClusterView) Fingerprint() string {
	if v == nil {
		return ""
	}
	return strings.Join([]string{v.version.Fingerprint(), v.members.Fingerprint()}, "|")
}
