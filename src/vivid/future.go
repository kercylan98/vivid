package vivid

import "fmt"

// TypedFutureFrom 创建一个类型化的 Future，
// 这个函数将一个通用的 Future 转换为一个类型化的 Future，提供类型安全的结果访问。
func TypedFutureFrom[M Message](f Future) TypedFuture[M] {
	return &typedFuture[M]{
		f: f,
	}
}

// TypedFuture 是一个类型化的 Future 接口，
// 它扩展了基本的 Future 接口，提供类型安全的结果访问。
type TypedFuture[M Message] interface {
	// Result 获取 Future 的结果，
	// 它会等待异步操作完成，并返回类型化的结果。
	Result() (m M, err error)

	// Wait 等待 Future 完成，
	// 它会等待异步操作完成，但不返回结果。
	Wait() (err error)

	// Close 关闭 Future，
	// 它会取消异步操作，并设置错误原因。
	Close(err error)
}

// typedFuture 是 TypedFuture 接口的实现，
// 它包装了一个基本的 Future，并提供类型安全的结果访问。
type typedFuture[M Message] struct {
	f Future // 内部的 Future 实例
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
