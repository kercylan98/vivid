package sugar

import (
	"fmt"
)

// With 创建一个携带指定值 value 和错误 err 的 Result[T] 泛型结果对象。
// value: 泛型参数，代表任意类型的结果值；err: 与该结果关联的错误。
// 常见应用场景包括自定义封装函数的返回值与错误，实现统一的错误处理与链式调用。
func With[T any](value T, err error) *Result[T] {
	return &Result[T]{
		value: value,
		err:   err,
	}
}

// Cast 对泛型结果 Result[T] 进行类型映射转换，输出类型为 Result[I]。
// 通常用于将携带具体类型的泛型结果向上转型（如结构体切换为接口），便于类型兼容和通用处理。
// 转换机制：
//  1. 尝试将 result.value 断言为目标类型 I。
//  2. 若断言成功，则返回新的 Result[I]，value 为转换后的结果，err 保持原始值；
//  3. 若断言失败（类型不兼容），则返回仅携带错误信息的 Result[I] 实例，明确指出转换失败原因。
//
// 注意事项：
//   - 仅适用于安全可断言类型间的转换（如结构体实现接口）；不支持不兼容类型。
//   - 调用方可据返回的错误判断转换是否成功，并据此分支处理业务逻辑。
func Cast[T any, I any](result *Result[T]) *Result[I] {
	i, ok := any(result.value).(I)
	if !ok {
		return Err[I](fmt.Errorf("cast failed, expected %T, got %T", i, result.value))
	}
	return &Result[I]{
		value: i,
		err:   result.err,
	}
}

// Err 创建一个仅包含错误 err 的 Result[T] 泛型结果对象。value 将采用零值。
// 适用于仅需传递错误而无需结果值的处理流程，如链式异常拦截、快速返回。
// err: 错误信息，若为 nil 表示无错；否则携带错误详情。
func Err[T any](err error) *Result[T] {
	return &Result[T]{
		err: err,
	}
}

// Ok 创建一个仅包含成功结果 v 的 Result[T]，err 字段为 nil。
// 用于表达无异常、仅需返回有效值的场景。
func Ok[T any](v T) *Result[T] {
	return &Result[T]{
		value: v,
	}
}

// None 创建一个不包含任何结果的 Result[T]，value 字段为零值，err 字段为 nil。
// 适用于需要明确表示“无结果”的场景，如空值、初始化等。
func None[T any]() *Result[T] {
	return &Result[T]{}
}

// Result 为通用结果封装结构体：携带一个类型为 T 的 value 结果值及对应的错误信息 err。
// 设计目的：
//   - 提高返回值与错误处理的一致性、可读性与健壮性。
//   - 配合泛型方法实现链式调用、流程编排以及友好的错误传递。
type Result[T any] struct {
	value T     // 泛型结果值，表示实际的数据内容
	err   error // 错误描述，为 nil 时表示操作成功
}

// Value 获取 Result[T] 中封装的原始结果值 value。
// 本方法不判断结果错误状态；调用者需结合 r.Err() 判断是否安全使用。
func (r *Result[T]) Value() T {
	return r.value
}

// Err 获取 Result[T] 关联的错误信息。
// 若无错误，返回 nil；否则返回具体错误对象。
// 建议优先判定该方法结果，决定是否进行后续处理。
func (r *Result[T]) Err() error {
	return r.err
}

// Unwrap 获取 Result[T] 的结果值。当 err 不为 nil 时，将 panic 抛出错误。
// 通常用于确定无错误分支的高效取值场景，非安全接口，建议保证 err 一定为 nil 时调用。
func (r *Result[T]) Unwrap() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.value
}

// UnwrapOr 在 Result[T] 无错时返回结果值 value，若有错则返回默认值 defaultValue。
// 适用于链式调用或无需捕捉错误时的兜底处理方式，保证调用不 panic。
func (r *Result[T]) UnwrapOr(defaultValue T) T {
	if r.err != nil {
		return defaultValue
	}
	return r.value
}

// IsOk 判定 Result[T] 是否为“成功”状态（即 err == nil）。
// 返回 true 表示操作没有错误，可放心使用结果值。
func (r *Result[T]) IsOk() bool {
	return r.err == nil
}

// IsErr 判定 Result[T] 是否为“错误”状态（即 err != nil）。
// 返回 true 表示有错误记录，调用方应根据 err 分支处理。
func (r *Result[T]) IsErr() bool {
	return r.err != nil
}

// Then 链式操作：若 Result[T] 无错（IsOk），则执行传入回调 fn 并返回执行结果；
// 若已有错，则直接返回当前 Result[T]，不中断错误传播。
// 主要用于按步骤处理依赖结果的流程控制，体现“短路”语义。
func (r *Result[T]) Then(fn func(ResultContainer[T], T) *Result[T]) *Result[T] {
	if r.IsOk() {
		return fn(ResultContainer[T]{}, r.value)
	}
	return r
}

// Else 链式错误处理：若 Result[T] 有错误（IsErr），调用 fn(err) 处理该错误并返回新结果；
// 否则保留现有结果。
// 适用于异常流程的补偿、日志记录或自定义错误转换。
func (r *Result[T]) Else(fn func(ResultContainer[T], error) *Result[T]) *Result[T] {
	if r.IsErr() {
		return fn(ResultContainer[T]{}, r.err)
	}
	return r
}

// Match 模式匹配与分支处理。根据 Result[T] 是否有错，选择执行对应分支：
//   - 成功（无错）：执行 success(value)；
//   - 失败（有错）：执行 failure(err)。
//
// 用于组织更复杂的处理逻辑，如将错误转化为特殊值、聚合统计等。
func (r *Result[T]) Match(success func(ResultContainer[T], T) *Result[T], failure func(error) *Result[T]) *Result[T] {
	if r.IsOk() {
		return success(ResultContainer[T]{}, r.value)
	}
	return failure(r.err)
}

// ResultPipeline 表示一个按序批量处理 Result[T] 的流水线。
// 可以逐步追加 Then 操作，最后一次性执行，提高复杂处理流程的可读性和一致性。
type ResultPipeline[T any] struct {
	result *Result[T]                               // 初始结果
	steps  []func(ResultContainer[T], T) *Result[T] // 处理步骤列表，每步接收 T，返回新的 Result[T]
}

// Then 向流水线追加后续步骤处理函数 fn。
// 每个 step 均检查上一步得到的结果是否无错，无错则执行 fn 否则直接传递原结果，支持链式调用。
func (p *ResultPipeline[T]) Then(fn func(ResultContainer[T], T) *Result[T]) *ResultPipeline[T] {
	p.steps = append(p.steps, func(container ResultContainer[T], t T) *Result[T] {
		if p.result.IsOk() {
			return fn(ResultContainer[T]{}, t)
		}
		return p.result
	})
	return p
}

// Execute 启动流水线，依次执行各步骤，传递并更新当前结果（可“短路”）。
// 返回执行后的最终 Result[T] 实例，调用方根据 IsOk/IsErr/Err/Value 等接口进一步判断和处理。
func (p *ResultPipeline[T]) Execute() *Result[T] {
	container := ResultContainer[T]{}
	for _, step := range p.steps {
		p.result = step(container, p.result.value)
	}
	return p.result
}
