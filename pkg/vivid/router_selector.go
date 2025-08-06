package vivid

import "math/rand/v2"

var (
	fanOutRouterSelector = RouterSelectorFN(func(tab RoutingTable) []ActorRef {
		return tab.GetAll()
	})

	randomRouterSelector = RouterSelectorFN(func(tab RoutingTable) []ActorRef {
		ee := tab.GetAll()
		return []ActorRef{ee[rand.IntN(len(ee))]}
	})
)

// RouterSelector 是用于从路由表中选择需要分发的 Actor 的选择器。
type RouterSelector interface {
	// Balance 从 tab 中选择处理消息的 Actor
	Balance(tab RoutingTable) []ActorRef
}

// RouterSelectorFN 是 RouterSelector 的函数式类型
type RouterSelectorFN func(tab RoutingTable) []ActorRef

func (fn RouterSelectorFN) Balance(tab RoutingTable) []ActorRef {
	return fn(tab)
}

// NewFanOutRouterSelector 此策略将给定的消息并行广播给所有可用的路由。
func NewFanOutRouterSelector() RouterSelector {
	return fanOutRouterSelector
}

// NewRandomRouterSelector 策略在其路由集合中随机选择一个路由并向其发送消息。
func NewRandomRouterSelector() RouterSelector {
	return randomRouterSelector
}

// NewRoundRobinRouterSelector 此策略以循环方式向其路由发送消息。对于通过路由器发送的 n 条消息，每个 Actor 都会转发一条消息。
//
// 该选择器是有状态的，应避免在多个 Actor 之间共享。
func NewRoundRobinRouterSelector() RouterSelector {
	var idx = 0
	return RouterSelectorFN(func(tab RoutingTable) []ActorRef {
		ee := tab.GetAll()
		e := []ActorRef{ee[idx]}
		idx++
		if idx == len(ee) {
			idx = 0
		}
		return e
	})
}
