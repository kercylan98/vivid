package actor_test

import (
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/stretchr/testify/assert"
)

func TestBehaviorStack_Push(t *testing.T) {
	stack := actor.NewBehaviorStack()

	var wg sync.WaitGroup
	wg.Add(1)
	stack.Push(func(ctx vivid.ActorContext) {
		wg.Done()
	})
	assert.Equal(t, 1, stack.Len())
	stack.Peek()(nil)
	wg.Wait()
}

func TestBehaviorStack_Pop(t *testing.T) {
	stack := actor.NewBehaviorStack()

	stack.Push(func(ctx vivid.ActorContext) {})
	assert.Equal(t, 1, stack.Len())
	assert.NotNil(t, stack.Pop())
	assert.Equal(t, 0, stack.Len())
}

func TestBehaviorStack_Clear(t *testing.T) {
	stack := actor.NewBehaviorStack()
	stack.Push(func(ctx vivid.ActorContext) {})
	stack.Clear()
	assert.Equal(t, 0, stack.Len())
}

func TestBehaviorStack_Len(t *testing.T) {
	stack := actor.NewBehaviorStack()
	stack.Push(func(ctx vivid.ActorContext) {})
	assert.Equal(t, 1, stack.Len())
}

func TestBehaviorStack_IsEmpty(t *testing.T) {
	stack := actor.NewBehaviorStack()
	assert.True(t, stack.IsEmpty())
}

func TestBehaviorStack_Peek(t *testing.T) {
	stack := actor.NewBehaviorStack()
	stack.Push(func(ctx vivid.ActorContext) {})
	assert.NotNil(t, stack.Peek())
}
