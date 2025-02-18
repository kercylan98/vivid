package vivid_test

import (
	vivid "github.com/kercylan98/vivid/src"
	"sync"
	"testing"
)

func TestActorContextTransportImplImpl_Tell(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
			}
		})
	})

	system.Tell(ref, "Hello")
}

func TestActorContextTransportImplImpl_TellRemote(t *testing.T) {
	system1 := vivid.NewActorSystem().StartP()
	system2 := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFn(func(config vivid.ActorSystemConfiguration) {
		config.WithListen(":8088")
	})).StartP()
	defer system1.ShutdownP()
	defer system2.ShutdownP()

	var wait sync.WaitGroup
	wait.Add(1)

	ref := system2.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
				wait.Done()
			}
		})
	})

	system1.Tell(ref, "Hello")

	wait.Wait()
}

func TestActorContextTransportImplImpl_Ask(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
				ctx.Reply("World")
			}
		})
	})

	result, err := system.Ask(ref, "Hello").Result()
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("Result", result)
}

func TestActorContextTransportImplImpl_AskRemote(t *testing.T) {
	system1 := vivid.NewActorSystem().StartP()
	system2 := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFn(func(config vivid.ActorSystemConfiguration) {
		config.WithListen(":0")
	})).StartP()
	defer system1.ShutdownP()
	defer system2.ShutdownP()

	ref := system2.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
				ctx.Reply("World")
			}
		})
	})

	result, err := system1.Ask(ref, "Hello").Result()
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("Result", result)
}

func TestActorContextTransportImplImpl_PoisonKill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	state := 0

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				if state != 0 {
					t.Error("State is not 0")
					return
				}
				state = 1
				t.Log("Receive", m)
			case vivid.OnKill:
				if state != 1 {
					t.Error("State is not 1")
					return
				}
				state = 2
				t.Log("OnKill")
			}
		})
	})

	system.Tell(ref, "Hello")
	system.PoisonKill(ref)
}

func TestActorContextTransportImplImpl_Kill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	state := 0

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				if state == 0 {
					t.Error("State is 0")
					return
				}
				state = 1
				t.Log("Receive", m)
			case vivid.OnKill:
				if state != 0 {
					t.Error("State is not 0")
					return
				}
				t.Log("OnKill")
			}
		})
	})

	system.Tell(ref, "Hello")
	system.Kill(ref)
}

func TestActorContextTransportImplImpl_RemoteKill(t *testing.T) {
	system1 := vivid.NewActorSystem().StartP()
	system2 := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFn(func(config vivid.ActorSystemConfiguration) {
		config.WithListen(":0")
	})).StartP()
	defer system1.ShutdownP()
	defer system2.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	ref := system2.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnKill:
				wait.Done()
			}
		})
	})

	system1.Kill(ref, "远程终止")
	wait.Wait()
}

func TestActorContextTransportImplImpl_RemotePoisonKill(t *testing.T) {
	system1 := vivid.NewActorSystem().StartP()
	system2 := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFn(func(config vivid.ActorSystemConfiguration) {
		config.WithListen(":0")
	})).StartP()
	defer system1.ShutdownP()
	defer system2.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	ref := system2.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnKill:
				wait.Done()
			}
		})
	})

	system1.PoisonKill(ref, "远程优雅终止")
	wait.Wait()
}

func TestActorContextTransportImplImpl_Watch(t *testing.T) {
	system1 := vivid.NewActorSystem().StartP()
	system2 := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFn(func(config vivid.ActorSystemConfiguration) {
		config.WithListen(":0")
	})).StartP()

	//defer system1.ShutdownP()
	defer system2.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	if err := system1.Watch(system2.Ref(), vivid.WatchHandlerFn(func(ctx vivid.ActorContext, stopped vivid.OnWatchStopped) {
		t.Log("watched stopped")
		wait.Done()
	})); err != nil {
		t.Error(err)
		return
	}

	system1.PoisonKill(system2.Ref(), "跨系统终止")
	wait.Wait()
}

func TestActorContextTransportImplImpl_Unwatch(t *testing.T) {
	system := vivid.NewActorSystem().StartP()

	defer system.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {})
	})

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnLaunch:
				if err := ctx.Watch(ref, vivid.WatchHandlerFn(func(ctx vivid.ActorContext, stopped vivid.OnWatchStopped) {
					t.Error("not should be called")
				})); err != nil {
					t.Error(err)
				}

				ctx.Unwatch(ref)
			}
		})
	})

	system.Kill(ref, "正常终止")
}

func TestActorContextTransportImplImpl_Restart(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case vivid.OnLaunch:
				if m.Restarted() {
					t.Log("Restarted")
				} else {
					t.Log("Launched")
				}
			}
		})
	})

	system.Restart(ref, false)
}

func TestActorContextTransportImplImpl_Broadcast(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	var wait sync.WaitGroup
	var refs []vivid.ActorRef

	for i := 0; i < 10; i++ {
		wait.Add(1)
		refs = append(refs, system.ActorOfFn(func() vivid.Actor {
			return vivid.ActorFn(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case string:
					wait.Done()
				}
			})
		}))
	}

	system.Broadcast("Hello")

	wait.Wait()
}
