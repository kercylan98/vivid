package vivid

import "fmt"

func TypedFutureFrom[M Message](f Future) TypedFuture[M] {
    return &typedFuture[M]{
        f: f,
    }
}

type TypedFuture[M Message] interface {
    Result() (m M, err error)

    Wait() (err error)

    Close(err error)
}

type typedFuture[M Message] struct {
    f Future
}

func (t *typedFuture[M]) Result() (m M, err error) {
    var result any
    result, err = t.f.Result()
    if err != nil {
        return m, err
    }
    m, ok := result.(M)
    if !ok {
        return m, fmt.Errorf("future result is not of type %T, got %T", m, result)
    }
    return m, nil
}

func (t *typedFuture[M]) Wait() (err error) {
    _, err = t.Result()
    return err
}

func (t *typedFuture[M]) Close(err error) {
    t.f.Close(err)
}
