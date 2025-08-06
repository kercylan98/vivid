package vivid

var _ RoutingTable = (*routerTable)(nil)

type RoutingTable interface {
	GetAll() []ActorRef
	GetMetrics(ref ActorRef) RouterMetrics
}

func newRouterTable(refs []ActorRef) *routerTable {
	tab := &routerTable{
		refs:    refs,
		metrics: make(map[ActorRef]*RouterMetrics),
	}
	for _, ref := range refs {
		tab.metrics[ref] = newRouterMetrics()
	}
	return tab
}

type routerTable struct {
	refs    []ActorRef
	metrics map[ActorRef]*RouterMetrics
}

func (r *routerTable) GetAll() []ActorRef {
	return r.refs
}

func (r *routerTable) GetMetrics(ref ActorRef) (metrics RouterMetrics) {
	if m, exist := r.metrics[ref]; exist {
		metrics = *m
	}
	return
}
