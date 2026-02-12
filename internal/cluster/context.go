package cluster

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/ves"
)

const (
	getViewTimeout = 3 * time.Second
)

// NewContext 根据已创建的 NodeActor 引用构造集群上下文。
// singletonNames 为已注册的集群单例名称集合，用于 SingletonRef 校验；nil 表示不校验。
func NewContext(system vivid.ActorSystem, clusterRef vivid.ActorRef, singletonNames []string) *Context {
	var names map[string]struct{}
	if len(singletonNames) > 0 {
		names = make(map[string]struct{}, len(singletonNames))
		for _, n := range singletonNames {
			names[n] = struct{}{}
		}
	}
	return &Context{
		system:         system,
		clusterRef:     clusterRef,
		singletonNames: names,
	}
}

// Context 是集群上下文，供 Actor 系统在运行时访问集群能力（如优雅退出、成员视图、多数派状态等）。
// 由 initializeCluster 链创建，未启用集群时为 nil。
type Context struct {
	system          vivid.ActorSystem
	clusterRef      vivid.ActorRef
	proxyManagerRef vivid.ActorRef
	singletonNames  map[string]struct{}
	leaveLock       sync.Mutex
	leaveWait       chan struct{}
}

// SetProxyManagerRef 设置集群单例代理管理器的 ActorRef，由 initializeCluster 在创建 ProxyManager 后调用。
func (c *Context) SetProxyManagerRef(ref vivid.ActorRef) {
	if c == nil {
		return
	}
	c.proxyManagerRef = ref
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
		info := vivid.ClusterMemberInfo{
			Address:    m.Address,
			Version:    strconv.FormatUint(ver, 10),
			Datacenter: m.Datacenter(),
			Rack:       m.Rack(),
			Region:     m.Region(),
			Zone:       m.Zone(),
		}
		if len(m.CustomState) > 0 {
			info.CustomState = make(map[string]string, len(m.CustomState))
			for k, v := range m.CustomState {
				info.CustomState[k] = v
			}
		}
		out = append(out, info)
	}
	return out, nil
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

// UpdateNodeState 将 customState 增量合并到本节点 NodeState.CustomState 并触发 Gossip 传播。
func (c *Context) UpdateNodeState(customState map[string]string) {
	if c == nil || c.clusterRef == nil || c.system == nil || len(customState) == 0 {
		return
	}
	c.system.Tell(c.clusterRef, &UpdateNodeStateRequest{CustomState: customState})
}

// getView 向 NodeActor 请求当前视图，用于 GetMembers、InQuorum。
func (c *Context) GetView() (*vivid.ClusterView, error) {
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
	return &vivid.ClusterView{
		LeaderAddr: resp.LeaderAddr,
		InQuorum:   resp.InQuorum,
	}, nil
}

// SingletonRef 返回名为 name 的集群单例的 ActorRef（本地代理）。
// 代理会订阅 Leader 变更并转发消息到当前单例，单例迁移后无需重新获取 ref；无可用单例时消息会缓存在代理中，待单例就绪后转发。
// 集群未启用、代理管理器未就绪或未配置该 name 的模板时返回错误。
func (c *Context) SingletonRef(name string) (vivid.ActorRef, error) {
	if c == nil || c.clusterRef == nil || c.system == nil {
		return nil, vivid.ErrorClusterDisabled
	}
	if c.proxyManagerRef == nil {
		return nil, vivid.ErrorClusterDisabled
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, vivid.ErrorIllegalArgument
	}
	if c.singletonNames != nil {
		if _, ok := c.singletonNames[name]; !ok {
			return nil, vivid.ErrorNotFound
		}
	}
	future := c.system.Ask(c.proxyManagerRef, &GetOrCreateProxyRequest{Name: name}, getViewTimeout)
	reply, err := future.Result()
	if err != nil {
		return nil, err
	}
	resp, ok := reply.(*GetOrCreateProxyResponse)
	if !ok || resp == nil {
		return nil, vivid.ErrorIllegalArgument
	}
	if resp.Err != nil {
		return nil, resp.Err
	}
	return resp.Ref, nil
}
