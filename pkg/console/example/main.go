package main

import (
	"errors"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/console"
)

func main() {
	system := bootstrap.NewActorSystem(vivid.WithActorSystemEnableMetrics(true))
	if err := system.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if err := system.Stop(); err != nil {
			panic(err)
		}
	}()

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch m := ctx.Message().(type) {
				case *vivid.OnLaunch:
					ctx.Scheduler().Loop(ctx.Ref(), time.Duration(rand.IntN(300)+1000)*time.Millisecond, 1)
					ctx.Scheduler().Once(ctx.Ref(), time.Duration(rand.IntN(1000)+1000)*time.Millisecond, errors.New("kill"))
				case *vivid.OnKill:
					ctx.Tell(ctx.Ref(), 1)
				case error:
					ctx.Kill(ctx.Ref(), false, m.Error())
				}
			}))
		}
	}()

	console.Serve(system, ":15800")

	systemSignal := make(chan os.Signal, 1)
	signal.Notify(systemSignal, syscall.SIGINT, syscall.SIGTERM)
	<-systemSignal
}
