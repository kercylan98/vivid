package vividkit

import (
	"fmt"
	"time"

	"github.com/kercylan98/vivid"
)

func Ask[T any](ctx vivid.ActorContext, recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) (result T, err error) {
	future := ctx.Ask(recipient, message, timeout...)
	futureResult, err := future.Result()
	if err != nil {
		return
	}
	if result, ok := futureResult.(T); !ok {
		return result, vivid.ErrorFutureMessageTypeMismatch.WithMessage(fmt.Sprintf("expected %T, got %T", result, futureResult))
	} else {
		return result, nil
	}
}
