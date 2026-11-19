package guard

import (
	"strconv"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

type stopTestRef struct {
	vivid.ActorRef
	path    vivid.ActorPath
	address string
}

func (r *stopTestRef) GetPath() vivid.ActorPath { return r.path }
func (r *stopTestRef) GetAddress() string       { return r.address }
func (r *stopTestRef) Equals(other vivid.ActorRef) bool {
	ref, ok := other.(*stopTestRef)
	return ok && r.path == ref.path && r.address == ref.address
}

type stopTestContext struct {
	vivid.ActorContext
	t                 *testing.T
	ref               vivid.ActorRef
	children          map[vivid.ActorPath]vivid.ActorRef
	kills             vivid.ActorRefs
	childrenSnapshots int
}

func newStopTestContext(t *testing.T, children ...vivid.ActorRef) *stopTestContext {
	ctx := &stopTestContext{t: t, ref: &stopTestRef{path: "/"}, children: make(map[vivid.ActorPath]vivid.ActorRef)}
	for _, child := range children {
		ctx.children[child.GetPath()] = child
	}
	return ctx
}

func (c *stopTestContext) Ref() vivid.ActorRef { return c.ref }
func (c *stopTestContext) Children() vivid.ActorRefs {
	c.childrenSnapshots++
	children := make(vivid.ActorRefs, 0, len(c.children))
	for _, child := range c.children {
		children = append(children, child)
	}
	return children
}
func (c *stopTestContext) Kill(ref vivid.ActorRef, poison bool, reason ...string) {
	require.False(c.t, poison, "关闭必须使用系统终止消息")
	require.Equal(c.t, []string{stopReason}, reason)
	c.kills = append(c.kills, ref)
}
func (c *stopTestContext) killed(actor *Actor, ref vivid.ActorRef) {
	delete(c.children, ref.GetPath())
	actor.onKilled(c, &vivid.OnKilled{Ref: ref})
}

func TestActor_StopPriorityOrder(t *testing.T) {
	business1, business2 := &stopTestRef{path: "/business-1"}, &stopTestRef{path: "/business-2"}
	gossip, remoting, metrics := &stopTestRef{path: "/gossip"}, &stopTestRef{path: "/remoting"}, &stopTestRef{path: "/metrics"}
	ctx := newStopTestContext(t, business1, business2, gossip, remoting, metrics)
	actor := NewActor(make(chan struct{}))
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: remoting, Priority: 1})
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: metrics, Priority: 0})
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: gossip, Priority: 2})
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: &stopTestRef{path: "/missing"}, Priority: 3})
	actor.onStop(ctx)
	require.ElementsMatch(t, vivid.ActorRefs{business1, business2}, ctx.kills)
	actor.onStop(ctx)
	require.Len(t, ctx.kills, 2, "重复停止不重复投递")

	// 重复、未知及同路径不同地址的通知不能减少待完成数量。
	actor.onKilled(ctx, &vivid.OnKilled{Ref: &stopTestRef{path: "/unknown"}})
	actor.onKilled(ctx, &vivid.OnKilled{Ref: &stopTestRef{path: business1.path, address: "other"}})
	ctx.killed(actor, business1)
	actor.onKilled(ctx, &vivid.OnKilled{Ref: business1})
	require.Len(t, ctx.kills, 2)
	ctx.killed(actor, business2)
	require.Equal(t, gossip, ctx.kills.Last())
	require.Len(t, ctx.kills, 3)
	actor.onKilled(ctx, &vivid.OnKilled{Ref: business2})
	require.Len(t, ctx.kills, 3, "当前组件结束前不能提前停止下一个组件")

	ctx.killed(actor, gossip)
	require.Equal(t, remoting, ctx.kills.Last())
	actor.onKilled(ctx, &vivid.OnKilled{Ref: gossip})
	require.Len(t, ctx.kills, 4)
	ctx.killed(actor, remoting)
	require.Equal(t, metrics, ctx.kills.Last())
	ctx.killed(actor, metrics)
	require.Equal(t, ctx.Ref(), ctx.kills.Last())
	require.Len(t, ctx.kills, 6)
	actor.onKilled(ctx, &vivid.OnKilled{Ref: metrics})
	require.Len(t, ctx.kills, 6)
	require.Equal(t, 1, ctx.childrenSnapshots)
	require.Nil(t, actor.pendingStops)
	require.Nil(t, actor.stopPriorities)
	require.Nil(t, actor.registerStopPriorities)
	require.Nil(t, actor.stoppingRegistered)
	actor.onKilled(ctx, &vivid.OnKilled{Ref: ctx.Ref()})
	select {
	case <-actor.guardClosedSignal:
	default:
		t.Fatal("根 Actor 收尾后必须通知系统")
	}
}

func TestActor_StopWithEarlyComponentDeath(t *testing.T) {
	for _, duringComponent := range []bool{false, true} {
		t.Run(strconv.FormatBool(duringComponent), func(t *testing.T) {
			business := &stopTestRef{path: "/business"}
			first, second := &stopTestRef{path: "/first"}, &stopTestRef{path: "/second"}
			ctx := newStopTestContext(t, business, first, second)
			actor := NewActor(make(chan struct{}))
			actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: first, Priority: 2})
			actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: second, Priority: 1})
			actor.onStop(ctx)
			queue := actor.stopPriorities
			if duringComponent {
				ctx.killed(actor, business)
				require.Equal(t, first, ctx.kills.Last())
				ctx.killed(actor, second)
				require.Len(t, ctx.kills, 2, "较低优先级提前死亡不能越过当前组件")
			} else {
				ctx.killed(actor, second)
				require.Len(t, ctx.kills, 1)
				ctx.killed(actor, business)
				require.Equal(t, first, ctx.kills.Last())
			}
			ctx.killed(actor, first)
			require.Equal(t, vivid.ActorRefs{business, first, ctx.Ref()}, ctx.kills)
			require.Equal(t, 1, ctx.childrenSnapshots)
			for _, priority := range queue {
				require.Nil(t, priority.ActorRef, "弹出后不保留组件引用")
			}
		})
	}
}

func TestActor_StopRegistrationSnapshot(t *testing.T) {
	first, second := &stopTestRef{path: "/first"}, &stopTestRef{path: "/second"}
	ctx := newStopTestContext(t, first, second)
	actor := NewActor(make(chan struct{}))
	registration := &RegisterStopPriority{ActorRef: first, Priority: 0}
	actor.onRegisterStopPriority(ctx, registration)
	registration.Priority = 3
	actor.onRegisterStopPriority(ctx, registration)
	registration.Priority = -1
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: second, Priority: 1})
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{})
	require.Len(t, actor.registerStopPriorities, 2)
	actor.onStop(ctx)
	require.Equal(t, vivid.ActorRefs{first}, ctx.kills, "按登记时复制的最新优先级停止")
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: second, Priority: 5})
	require.Nil(t, actor.registerStopPriorities, "开始关闭后冻结登记")
	ctx.killed(actor, first)
	require.Equal(t, vivid.ActorRefs{first, second}, ctx.kills)
	ctx.killed(actor, second)
	require.Equal(t, ctx.Ref(), ctx.kills.Last())
}

func TestActor_StopWithStaleRegistration(t *testing.T) {
	child := &stopTestRef{path: "/child"}
	ctx := newStopTestContext(t, child)
	actor := NewActor(make(chan struct{}))
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: child})
	actor.onKilled(ctx, &vivid.OnKilled{Ref: &stopTestRef{path: child.path, address: "other"}})
	require.Len(t, actor.registerStopPriorities, 1)
	ctx.killed(actor, child)
	require.Empty(t, actor.registerStopPriorities, "正常运行时移除已死亡组件的登记")
	actor.onRegisterStopPriority(ctx, &RegisterStopPriority{ActorRef: &stopTestRef{path: "/missing"}})
	actor.onStop(ctx)
	require.Equal(t, vivid.ActorRefs{ctx.Ref()}, ctx.kills)
	require.Nil(t, actor.registerStopPriorities)
	require.Nil(t, actor.pendingStops)
}

func TestActor_StopManyChildren(t *testing.T) {
	const count = 10000
	children := make(vivid.ActorRefs, count)
	for index := range children {
		children[index] = &stopTestRef{path: "/" + strconv.Itoa(index)}
	}
	ctx := newStopTestContext(t, children...)
	actor := NewActor(make(chan struct{}))
	actor.onStop(ctx)
	require.Len(t, ctx.kills, count)
	for _, child := range children {
		ctx.killed(actor, child)
	}
	require.Len(t, ctx.kills, count+1)
	require.Equal(t, ctx.Ref(), ctx.kills.Last())
	require.Equal(t, 1, ctx.childrenSnapshots, "终止通知不能重复取得子节点快照")
	require.Nil(t, actor.pendingStops)
}
