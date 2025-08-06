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

type RouterSelector interface {
	Balance(tab RoutingTable) []ActorRef
}

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
