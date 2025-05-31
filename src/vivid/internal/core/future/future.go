package future

type Future interface {
    // Close 提前关闭 Future
    Close(err error)

    // Result 获取 Future 的结果，如果 Future 未关闭则会阻塞
    Result() (any, error)

    // Wait 等待 Future 关闭，如果 Future 未关闭则会阻塞
    Wait() error
}
