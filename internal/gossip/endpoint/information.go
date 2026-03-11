package endpoint

import (
	"time"

	"github.com/kercylan98/vivid"
)

// NewInformation 构造本节点或对端节点的端点信息，初始状态为 StatusNone。
func NewInformation(actorRef vivid.ActorRef) *Information {
	return &Information{
		Status:   StatusNone,
		LastSeen: time.Now(),
		ActorRef: actorRef,
	}
}

// Information 单节点的端点信息，随 Ping/Pong 在集群内传播，用于成员列表与状态展示。
type Information struct {
	ActorRef vivid.ActorRef // 节点对应的 Actor 引用，用于通信与唯一标识
	Status   Status         // 当前生命周期状态（None/Joining/Up）
	LastSeen time.Time      // 最近一次被本节点观察到的时间，可用于后续故障检测
}

// ID 返回节点唯一标识，与 ActorRef.String() 一致，用作成员列表与版本向量的 key。
func (i *Information) ID() string {
	return i.ActorRef.String()
}
