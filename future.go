package vivid

import "errors"

var (
	ErrFutureTimeout             = errors.New("future timeout")
	ErrFutureMessageTypeMismatch = errors.New("future message type mismatch")
)

type Future[T any] interface {
	Close(err error)
	Result() (T, error)
	Wait() error
}
