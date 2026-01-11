package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
)

func TestScheduler_Cron(t *testing.T) {
	system := NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			assert.NoError(t, ctx.Scheduler().Cron(ctx.Ref(), "* * * * * *", 1))
		case int:
			wg.Done()
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	wg.Wait()
}

func TestScheduler_Once(t *testing.T) {
	system := NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			assert.NoError(t, ctx.Scheduler().Once(ctx.Ref(), 1*time.Second, 1))
		case int:
			wg.Done()
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	wg.Wait()
}

func TestScheduler_Loop(t *testing.T) {
	system := NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var count int32
	var wg sync.WaitGroup
	wg.Add(3)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			assert.NoError(t, ctx.Scheduler().Loop(ctx.Ref(), 100*time.Microsecond, 1))
		case int:
			count++
			if count == 3 {
				ctx.Scheduler().Clear()
			}
			wg.Done()
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	wg.Wait()
}
