// Package future 提供了异步操作的 Future 模式实现。
//
// Future 模式用于处理异步操作的结果，允许调用者在不阻塞的情况下发起操作，
// 并在需要时获取操作结果。这在 Actor 系统中用于实现 Ask 模式的消息传递。
package future

import "fmt"

var (
    ErrorFutureTimeout = fmt.Errorf("future timeout")
)

// Future 定义了异步操作结果的接口。
//
// Future 代表一个可能尚未完成的异步操作的结果。
// 它提供了检查操作是否完成、获取结果或等待完成的方法。
//
// 典型用法：
//   - 发起异步操作并获得 Future
//   - 继续执行其他工作
//   - 在需要时调用 Result() 获取结果
//   - 或调用 Wait() 仅等待操作完成
type Future interface {
    // Close 提前关闭 Future 并设置错误。
    //
    // 当需要取消异步操作或设置错误状态时使用此方法。
    // 调用此方法后，任何等待此 Future 的操作都会收到指定的错误。
    //
    // 参数 err 是要设置的错误，如果为 nil 则表示正常关闭。
    Close(err error)

    // Result 获取 Future 的结果。
    //
    // 如果异步操作尚未完成，此方法会阻塞直到操作完成。
    // 返回操作的结果和可能的错误。
    //
    // 返回值：
    //   - any: 异步操作的结果，类型取决于具体的操作
    //   - error: 操作过程中发生的错误，如果操作成功则为 nil
    Result() (any, error)

    // Wait 等待 Future 完成。
    //
    // 与 Result() 不同，此方法只等待操作完成但不返回结果。
    // 如果只关心操作是否完成而不需要结果时，使用此方法更高效。
    //
    // 返回操作过程中发生的错误，如果操作成功则为 nil。
    Wait() error
}
