package vivid

import (
	"fmt"
	"github.com/kercylan98/vivid/core/vivid/future"
	"time"
)

func TypedAsk[T Message](context ActorContext, target ActorRef, message Message, timeout ...time.Duration) *TypedFuture[T] {
	return &TypedFuture[T]{Future: context.Ask(target, message, timeout...)}
}

type TypedFuture[T Message] struct {
	future.Future
}

func (t *TypedFuture[T]) Result() (v T, err error) {
	result, err := t.Future.Result()
	if err != nil {
		return v, err
	}
	if typed, ok := result.(T); !ok {
		return v, fmt.Errorf("unexpected result type: %T", result)
	} else {
		return typed, nil
	}
}
