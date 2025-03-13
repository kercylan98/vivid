package vivid_test

import (
    "fmt"
    "github.com/kercylan98/vivid/src/vivid"
    "testing"
)

func TestActorSystem_ActorOf(t *testing.T) {
    system := vivid.NewActorSystem().StartP()

    ref := system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
        return vivid.ActorFN(func(ctx vivid.ActorContext) {
            fmt.Println(1)
        })
    }))

    t.Log(ref.Address(), ref.Path())
}
