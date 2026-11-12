package future_test

import (
	"context"
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/stretchr/testify/require"
)

type forwardRef struct{ vivid.ActorRef }

func (r *forwardRef) String() string     { return "subscriber" }
func (r *forwardRef) GetAddress() string { return "local" }
func (r *forwardRef) GetPath() string    { return "/subscriber" }

type forwardLiaison struct {
	vivid.ActorLiaison
	results chan *vivid.PipeResult
}

func (l *forwardLiaison) Tell(_ vivid.ActorRef, message vivid.Message) {
	l.results <- message.(*vivid.PipeResult)
}

func TestFuture_ConcurrentCompletionAndPipeTo(t *testing.T) {
	for _, fail := range []bool{false, true} {
		for range 2000 {
			liaison := &forwardLiaison{results: make(chan *vivid.PipeResult, 1)}
			f := future.NewFuture[string](liaison, 0, nil)
			var start sync.WaitGroup
			start.Add(1)
			var completed sync.WaitGroup
			completed.Add(2)
			go func() {
				defer completed.Done()
				start.Wait()
				f.PipeTo(vivid.ActorRefs{&forwardRef{}})
			}()
			go func() {
				defer completed.Done()
				start.Wait()
				if fail {
					f.Close(context.Canceled)
				} else {
					f.EnqueueMessage("ready")
				}
			}()
			start.Done()
			completed.Wait()
			message := <-liaison.results
			if fail {
				require.ErrorIs(t, message.Error, context.Canceled)
			} else {
				require.NoError(t, message.Error)
				require.Equal(t, "ready", message.Message)
			}
			require.Empty(t, liaison.results, "每个目标仅转发一次完成结果")
		}
	}
}
