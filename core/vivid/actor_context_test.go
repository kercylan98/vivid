package vivid_test

import (
	"github.com/kercylan98/vivid/core/vivid"
	"sync/atomic"
	"testing"
)

func TestActorContext_AttachTask(t *testing.T) {
	NewTestActorSystem(t).
		WaitAdd(1).
		WaitFN(func(system *TestActorSystem) {
			system.SpawnOf(func() vivid.Actor {
				return vivid.ActorFN(func(context vivid.ActorContext) {
					switch context.Message().(type) {
					case *vivid.OnLaunch:
						context.AttachTask(vivid.ActorContextTaskFN(func(context vivid.TaskContext) {
							switch context.Message().(type) {
							case *vivid.OnLaunch:
								system.WaitDone()
							}
						}))
					}
				})
			})
		}).
		Shutdown(true)
}

func TestActorContext_Tell(t *testing.T) {
	NewTestActorSystem(t).
		WaitAdd(2).
		WaitFN(func(system *TestActorSystem) {
			ref := system.SpawnOf(func() vivid.Actor {
				return vivid.ActorFN(func(context vivid.ActorContext) {
					switch context.Message().(type) {
					case *vivid.OnLaunch:
						system.WaitDone()
					case string:
						system.AssertNil(context.Sender())
						system.WaitDone()
					}
				})
			})

			system.Tell(ref, t.Name())
		}).
		Shutdown(true)
}

func BenchmarkActorContext_Tell(b *testing.B) {
	system := vivid.NewActorSystemWithOptions()
	defer func() {
		if err := system.Shutdown(false); err != nil {
			b.Error(err)
		}
	}()
	ref := system.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, i)
	}
	b.StopTimer()
}

func TestActorContext_Probe(t *testing.T) {
	NewTestActorSystem(t).
		WaitAdd(2).
		WaitFN(func(system *TestActorSystem) {
			ref := system.SpawnOf(func() vivid.Actor {
				return vivid.ActorFN(func(context vivid.ActorContext) {
					switch context.Message().(type) {
					case *vivid.OnLaunch:
						system.WaitDone()
					case string:
						system.AssertNotNil(context.Sender())
						system.WaitDone()
					}
				})
			})

			system.Probe(ref, t.Name())
		}).
		Shutdown(true)
}

func TestActorContext_Ask(t *testing.T) {
	NewTestActorSystem(t).
		WaitAdd(2).
		WaitFN(func(system *TestActorSystem) {
			ref := system.SpawnOf(func() vivid.Actor {
				return vivid.ActorFN(func(context vivid.ActorContext) {
					switch context.Message().(type) {
					case *vivid.OnLaunch:
						system.WaitDone()
					case string:
						context.Reply(t.Name())
						system.WaitDone()
					}
				})
			})

			future := system.Ask(ref, t.Name())
			n, err := future.Result()
			system.AssertError(err)
			system.AssertEqual(n, t.Name())
		}).
		Shutdown(true)
}

func TestActorContext_Kill(t *testing.T) {
	var counter atomic.Int64

	NewTestActorSystem(t).
		WaitAdd(1).
		WaitFN(func(system *TestActorSystem) {
			ref := system.SpawnOf(func() vivid.Actor {
				return vivid.ActorFN(func(context vivid.ActorContext) {
					switch context.Message().(type) {
					case string:
						counter.Add(1)
					case *vivid.OnKill:
						system.WaitDone()
					}
				})
			})

			for i := 0; i < 10; i++ {
				system.Tell(ref, t.Name())
			}

			system.Kill(ref)
		}).
		Shutdown(true)

	if counter.Load() == 10 {
		// Kill 情况下无法将所有用户消息完整处理
		t.Error("kill failed, counter: ", counter.Load())
	} else {
		t.Log("kill success, counter: ", counter.Load())
	}
}

func TestActorContext_PoisonKill(t *testing.T) {
	NewTestActorSystem(t).
		WaitAdd(2).
		WaitFN(func(system *TestActorSystem) {
			ref := system.SpawnOf(func() vivid.Actor {
				return vivid.ActorFN(func(context vivid.ActorContext) {
					switch context.Message().(type) {
					case *vivid.OnLaunch:
						system.WaitDone()
						const childNum = 10
						for i := 0; i < childNum; i++ {
							system.WaitAdd(1)
							context.SpawnOf(func() vivid.Actor {
								return vivid.ActorFN(func(context vivid.ActorContext) {})
							})
						}
					case *vivid.OnKill, *vivid.OnKilled:
						system.WaitDone()
					}
				})
			})

			system.PoisonKill(ref)
		}).
		Shutdown(true)
}

func TestActorContext_Watch(t *testing.T) {
	system := NewTestActorSystem(t).WaitAdd(1)
	ref := system.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case *vivid.OnKill:
				t.Log("kill")
			}
		})
	})

	system.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case *vivid.OnLaunch:
				context.Watch(ref)
				context.PoisonKill(ref)
			case *vivid.OnWatchEnd:
				t.Log("watch end")
				system.WaitDone()
			}
		})
	})

	system.Wait()
	system.Shutdown(true)
}
