package vivid_test

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"testing"
)

var _ vivid.RoutingTable = (*TestRouterTable)(nil)

func NewTestRouterTable() *TestRouterTable {
	return &TestRouterTable{
		Refs: []vivid.ActorRef{
			vivid.NewActorRef("127.0.0.1:1000", "1"),
			vivid.NewActorRef("127.0.0.1:1000", "2"),
			vivid.NewActorRef("127.0.0.1:1000", "3"),
		},
	}
}

type TestRouterTable struct {
	Refs []vivid.ActorRef
}

func (t *TestRouterTable) GetAll() []vivid.ActorRef {
	return t.Refs
}

func (t *TestRouterTable) GetMetrics(ref vivid.ActorRef) vivid.RouterMetrics {
	return vivid.RouterMetrics{}
}

func TestSelectors(t *testing.T) {
	tab := NewTestRouterTable()
	t.Run("FanOut", func(t *testing.T) {
		selector := vivid.NewFanOutRouterSelector()
		refs := selector.Balance(tab)
		if len(refs) != len(tab.Refs) {
			t.Errorf("len(refs) = %d; want %d", len(refs), len(tab.Refs))
		}
	})

	t.Run("Random", func(t *testing.T) {
		selector := vivid.NewRandomRouterSelector()
		refs := selector.Balance(tab)
		if len(refs) != 1 {
			t.Errorf("len(refs) = %d; want %d", len(refs), 1)
		}
	})

	t.Run("RoundRobin", func(t *testing.T) {
		selector := vivid.NewRoundRobinRouterSelector()
		records := make(map[vivid.ActorRef]int)
		for i := 0; i < len(tab.Refs)*10; i++ {
			refs := selector.Balance(tab)
			for _, ref := range refs {
				records[ref]++
			}
		}

		curr := -1
		for _, i := range records {
			if curr == -1 {
				curr = i
			} else if curr != i {
				t.Errorf("except = %d; got %d", curr, i)
				return
			}
		}
	})
}
