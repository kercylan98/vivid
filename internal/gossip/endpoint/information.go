package endpoint

import (
	"time"

	"github.com/google/uuid"
	"github.com/kercylan98/vivid"
)

// NewInformation 为本次启动创建独立身份，初始状态为 StatusNone；转发与复制应保留原实例 ID。
func NewInformation(actorRef vivid.ActorRef, createdAt time.Time) *Information {
	return &Information{
		InstanceID: uuid.NewString(),
		Status:     StatusNone,
		ActorRef:   actorRef,
		CreatedAt:  createdAt,
	}
}

// Information 单节点的端点信息，随 Ping/Pong 在集群内传播，用于成员列表与状态展示。
type Information struct {
	InstanceID string         // 本次启动的实例 ID，不用于判断新旧或授予接管资格
	ActorRef   vivid.ActorRef // 节点对应的 Actor 引用，仅用于通信
	Status     Status         // 当前生命周期状态
	CreatedAt  time.Time      // 节点创建时间，用于计算协调者节点 ID
}

// Clone 深拷贝当前端点信息，保留实例身份。
func (i *Information) Clone() *Information {
	return &Information{
		InstanceID: i.InstanceID,
		ActorRef:   i.ActorRef.Clone(),
		Status:     i.Status,
		CreatedAt:  i.CreatedAt,
	}
}

// ID 返回本次启动的实例标识，用作成员列表与版本向量的 key。
func (i *Information) ID() string {
	return i.InstanceID
}
