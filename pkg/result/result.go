package result

import "fmt"

// With 返回一个包含指定 value 和 err 的 Result[T] 实例。
// value 为任意类型的结果值，err 为可能产生的错误。
// 常用于包装函数返回值和错误，实现一致的错误处理流程。
func With[T any](value T, err error) *Result[T] {
	return &Result[T]{
		value: value,
		err:   err,
	}
}

// Cast 将一个 Result[T] 泛型结果类型转换为结果类型为 Result[I] 的实例。
// 该方法常用于类型兼容或升级，如将更具体的类型 T 转换为其接口类型 I。
//
// 转换过程：
//   1. 使用 Go 的类型断言机制（any(result.value).(I)），尝试将原 Result[T] 保存的值转换为类型 I。
//   2. 如果断言成功，则将转换后的值和原有的错误（err）封装到新的 Result[I] 返回，保持错误信息不丢失。
//   3. 如果断言失败（T 不能赋值给 I），则返回一个仅包含错误信息的 Result[I]，错误内容指明转换源与目标类型。
//
// 使用场景：
//   - 泛型代码或库中需要向上传递具体类型向接口类型的结果
//   - 代码链路中接口升级、类型动态切换时安全携带错误处理能力
//
// 注意：类型不兼容时只返回错误，不传递原始值，调用方应处理可能的失败情况。
func Cast[T any, I any](result *Result[T]) *Result[I] {
	i, ok := any(result.value).(I)
	if !ok {
		return Error[I](fmt.Errorf("cast failed, expected %T, got %T", i, result.value))
	}
	return &Result[I]{
		value: i,
		err:   result.err,
	}
}

// Error 返回一个仅包含错误的 Result[T] 实例。
// 常用于只需传递错误，而无需结果值的场景。
func Error[T any](err error) *Result[T] {
	return &Result[T]{
		err: err,
	}
}

// Result 封装了一个泛型结果 value 及相关的错误信息 err。
// 通常用于统一处理函数的返回值和错误状态，提高调用的可读性和健壮性。
type Result[T any] struct {
	value T     // 结果值
	err   error // 错误信息
}

// Value 返回 Result 中封装的结果值。
// 不判断 err 状态，调用方应根据需求自行判断。
func (r *Result[T]) Value() T {
	return r.value
}

// Error 返回 Result 中的错误信息。
// 如果无错误发生，返回值为 nil。
func (r *Result[T]) Error() error {
	return r.err
}

// Unwrap 返回结果值 value。如果 Result 包含错误（err 不为 nil），将触发 panic。
// 适合明确保证无错误结果的场景，否则建议使用更安全的调用方法。
func (r *Result[T]) Unwrap() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.value
}

// UnwrapOr 返回结果值 value。如果 Result 包含错误（err 不为 nil），则返回指定的默认值 defaultValue。
// 适用于无需处理错误但需要兜底值的调用场景。
func (r *Result[T]) UnwrapOr(defaultValue T) T {
	if r.err != nil {
		return defaultValue
	}
	return r.value
}

// IsOk 返回 Result 是否包含错误。
// 如果无错误发生，返回值为 true。
func (r *Result[T]) IsOk() bool {
	return r.err == nil
}

// IsErr 返回 Result 是否包含错误。
// 如果包含错误，返回值为 true。
func (r *Result[T]) IsErr() bool {
	return r.err != nil
}
