package cluster

import (
	"strconv"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/ves"
)

const (
	getViewTimeout = 3 * time.Second
)

// NewContext 根据已创建的 NodeActor 引用构造集群上下文。
func NewContext(system vivid.ActorSystem, clusterRef vivid.ActorRef) *Context {
	return &Context{system: system, clusterRef: clusterRef}
}

// Context 是集群上下文，供 Actor 系统在运行时访问集群能力（如优雅退出、成员视图、多数派状态等）。
// 由 initializeCluster 链创建，未启用集群时为 nil。
type Context struct {
	system     vivid.ActorSystem
	clusterRef vivid.ActorRef
	leaveLock  sync.Mutex
	leaveWait  chan struct{}
}

// GetMembers 返回当前视图中的成员列表；未启用集群或 clusterRef 为空时返回 ErrorClusterDisabled。
func (c *Context) GetMembers() ([]vivid.ClusterMemberInfo, error) {
	if c == nil || c.clusterRef == nil || c.system == nil {
		return nil, vivid.ErrorClusterDisabled
	}
	future := c.system.Ask(c.clusterRef, &GetViewRequest{}, getViewTimeout)
	reply, err := future.Result()
	if err != nil {
		return nil, err
	}
	resp, ok := reply.(*GetViewResponse)
	if !ok || resp == nil || resp.View == nil {
		return nil, vivid.ErrorIllegalArgument
	}
	out := make([]vivid.ClusterMemberInfo, 0, len(resp.View.Members))
	for _, m := range resp.View.Members {
		if m == nil {
			continue
		}
		ver := resp.View.VersionVector.Get(m.ID)
		out = append(out, vivid.ClusterMemberInfo{
			Address:    m.Address,
			Version:    strconv.FormatUint(ver, 10),
			Datacenter: m.Datacenter(),
			Rack:       m.Rack(),
			Region:     m.Region(),
			Zone:       m.Zone(),
		})
	}
	return out, nil
}

// InQuorum 返回当前节点是否处于多数派（健康数 >= 法定人数）；未启用集群时返回 ErrorClusterDisabled。
func (c *Context) InQuorum() (bool, error) {
	if c == nil || c.clusterRef == nil || c.system == nil {
		return false, vivid.ErrorClusterDisabled
	}
	future := c.system.Ask(c.clusterRef, &GetViewRequest{}, getViewTimeout)
	reply, err := future.Result()
	if err != nil {
		return false, err
	}
	resp, ok := reply.(*GetViewResponse)
	if !ok || resp == nil || resp.View == nil {
		return false, vivid.ErrorIllegalArgument
	}
	return resp.InQuorum, nil
}

// Leave 向集群节点发送优雅退出请求，并等待「已退出」后再返回。
// 内部会临时启动一个 Actor 监听 ClusterLeaveCompletedEvent，收到事件后解除阻塞；若超时则直接返回，不阻塞 Stop 流程。只执行一次，幂等。
// 若未启用集群或 clusterRef 为空则直接返回。
func (c *Context) Leave() {
	if c == nil || c.clusterRef == nil || c.system == nil {
		return
	}

	c.leaveLock.Lock()
	if c.leaveWait == nil {
		c.leaveWait = make(chan struct{})
		_, _ = c.system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.EventStream().Subscribe(ctx, ves.ClusterLeaveCompletedEvent{})
				ctx.Tell(c.clusterRef, &LeaveRequest{})
			case ves.ClusterLeaveCompletedEvent:
				close(c.leaveWait)
			}
		}))
	}
	c.leaveLock.Unlock()
	<-c.leaveWait
}
