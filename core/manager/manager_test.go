package manager_test

import (
	"github.com/kercylan98/vivid/core"
	"github.com/kercylan98/vivid/core/manager"
	"github.com/kercylan98/vivid/core/server"
	"testing"
	"time"
)

func TestOptions_WithServer(t *testing.T) {
	mgr := manager.Builder().ConfiguratorOf(core.FnManagerConfigurator(func(options core.ManagerOptions) {
		options.WithServer(
			server.Builder().Build(),
		)
	}))

	if err := mgr.Run(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
}
