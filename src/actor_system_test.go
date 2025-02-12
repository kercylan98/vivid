package vivid_test

import (
	vivid "github.com/kercylan98/vivid/src"
	"testing"
)

func TestNewActorSystem(t *testing.T) {
	var cases = []struct {
		name   string
		config vivid.ActorSystemConfiguration
		err    bool
	}{
		{"标准默认配置", vivid.NewActorSystemConfig(), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sys := vivid.GetActorSystemBuilder().FromConfiguration(c.config)
			if sys == nil {
				if !c.err {
					t.Error("创建 ActorSystem 失败")
				}
				return
			}
			if err := sys.Start(); err != nil && !c.err {
				t.Error("启动 ActorSystem 失败", err)
				return
			}
			if err := sys.Shutdown(); err != nil && !c.err {
				t.Error("关闭 ActorSystem 失败", err)
				return
			}

			if c.err {
				t.Error("创建 ActorSystem 失败，应有错误但未发现")
			}
		})
	}
}
