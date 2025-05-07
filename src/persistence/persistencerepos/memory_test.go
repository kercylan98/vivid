package persistencerepos_test

import (
	"github.com/kercylan98/vivid/src/persistence"
	"github.com/kercylan98/vivid/src/persistence/persistencerepos"
	"testing"
)

func TestMemory_SaveAndLoad(t *testing.T) {
	var repo = persistencerepos.NewMemory()
	var state = persistence.NewState("test", repo)

	state.Update(1)
	state.Update(2)
	state.Update(3)
	if err := state.Persist(); err != nil {
		t.Error(err)
	}

	state = persistence.NewState("test", repo)
	if err := state.Load(); err != nil {
		t.Error(err)
	}

	if len(state.GetEvents()) != 3 {
		t.Error("state events length error")
	}

	state.SaveSnapshot(999)
	if err := state.Persist(); err != nil {
		t.Error(err)
	}

	state = persistence.NewState("test", repo)

	if err := state.Load(); err != nil {
		t.Error(err)
	}

	if state.GetSnapshot() != 999 {
		t.Error("state snapshot error")
	}

	if len(state.GetEvents()) != 0 {
		t.Error("state events length error")
	}
}
