package manager_test

import (
	"github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/manager"
	"github.com/kercylan98/vivid/.discord/src/transport/server"
	"testing"
	"time"
)

func TestOptions_WithServer(t *testing.T) {
	mgr := manager.Builder().ConfiguratorOf(vivid.FnManagerConfigurator(func(options vivid.ManagerOptions) {
		options.WithAddrServer(
			server.Builder().Build(),
		)
	}))

	if err := mgr.Run(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
}
