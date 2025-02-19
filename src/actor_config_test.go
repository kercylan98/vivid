package vivid_test

import (
	vivid "github.com/kercylan98/vivid/src"
	"strings"
	"testing"
	"time"
)

func TestDefaultActorConfig_WithName(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnLaunch:
				if !strings.Contains(ctx.Ref().String(), "my-name") {
					t.Error("Actor name should be set")
				}
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithName("my-name")
	})
}

func TestDefaultActorConfig_WithReadOnly(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case vivid.OnLaunch:
				if _, exist := m.GetContext("123"); exist {
					t.Error("ReadOnly context should not be modified")
				}
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithReadOnly().WithLaunchContextProvider(vivid.LaunchContextProviderFn(func() map[any]any {
			return map[any]any{"123": "456"}
		}))
	})
}

func TestDefaultActorConfig_Logic(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {})
	}, func(config vivid.ActorConfiguration) {
		config.WithName("123").If(func(config vivid.ActorOptionsFetcher) bool {
			return config.FetchName() == "123"
		}, func(options vivid.ActorOptions) {
			options.WithName("456")
		})
	})
}

func TestDefaultActorConfig_WithSlowMessageThreshold(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnLaunch:
				time.Sleep(time.Second)
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSlowMessageThreshold(time.Millisecond * 900)
	})
}
