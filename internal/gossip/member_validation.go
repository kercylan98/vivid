package gossip

import (
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/utils"
)

// validMember 校验可传播的成员记录，同一实例的固定声明不变，生命周期只能保持或前进。
// None 仅用于本地启动；转发记录无效时连同对应版本一起忽略，不阻断其余记录。
func (a *Actor) validMember(info *endpoint.Information) bool {
	if info == nil || info.ActorRef == nil || info.ID() == "" ||
		info.Status < endpoint.StatusJoining || info.Status > endpoint.StatusRemoved ||
		info.ActorRef.IsVirtual() || info.ActorRef.GetPath() != "/"+gossipActorName {
		return false
	}
	address := info.ActorRef.GetAddress()
	known := a.view.Members().Get(info.ID())
	// 已接收实例复核固定声明，地址格式无需在每次状态传播时重新解析。
	if known != nil {
		// 传播可能跳过中间阶段，但更高版本也不能恢复已退出的实例。
		return info.Status >= known.Status && known.ActorRef.GetAddress() == address && known.DataCenter == info.DataCenter && known.CreatedAt.Equal(info.CreatedAt)
	}
	if info.DataCenter != "" && !utils.IsValidClusterLabel(info.DataCenter) {
		return false
	}
	normalized, ok := utils.NormalizeRemoteAddress(address)
	return ok && normalized == address
}
