// Package processor 提供了处理单元的核心接口定义。
package processor

// Unit 定义了处理器单元的基本接口。
// 处理器单元是系统中用于处理消息的基本组件，所有的业务逻辑都通过处理单元来实现。
// 每个处理单元都有唯一的标识符，并能够处理来自其他单元发送的消息。
type Unit interface {
    // HandleUserMessage 处理来自指定发送者的消息。
    // sender 参数标识消息的发送者，如果为 nil 则表示系统消息。
    // message 参数是要处理的消息内容，可以是任意类型。
    // 实现者应该根据消息类型进行相应的处理逻辑。
    HandleUserMessage(sender UnitIdentifier, message any)

    // HandleSystemMessage 处理来自系统自身的消息。
    // message 参数是要处理的消息内容，可以是任意类型。
    // 实现者应该根据消息类型进行相应的处理逻辑。
    HandleSystemMessage(sender UnitIdentifier, message any)
}

// UnitInitializer 定义了具有初始化能力的处理单元接口。
// 继承了 Unit 接口的所有功能，并提供了初始化方法。
// 当处理单元被注册到注册表时，如果实现了此接口，系统会自动调用 Init 方法。
type UnitInitializer interface {
    Unit

    // Init 初始化处理单元。
    // 该方法在处理单元被注册时自动调用，用于执行必要的初始化操作。
    // 实现者可以在此方法中设置初始状态、建立连接、分配资源等。
    Init()
}

// asUnitInitializer 将 Unit 转换为 UnitInitializer 接口。
// 如果给定的 unit 实现了 UnitInitializer 接口，则返回对应的接口实例。
// 如果未实现该接口，则返回 nil。
// 该函数主要用于注册表内部的类型检查和转换。
func asUnitInitializer(unit Unit) UnitInitializer {
    if initializer, ok := unit.(UnitInitializer); ok {
        return initializer
    }
    return nil
}

// UnitCloser 定义了具有关闭能力的处理单元接口。
// 继承了 Unit 接口的所有功能，并提供了关闭和状态检查方法。
// 当处理单元从注册表中注销时，如果实现了此接口，系统会自动调用 Close 方法。
type UnitCloser interface {
    Unit

    // Close 关闭处理单元。
    // operator 参数标识执行关闭操作的操作者，用于审计和日志记录。
    // 该方法在处理单元被注销时自动调用，用于执行清理操作。
    // 实现者可以在此方法中释放资源、关闭连接、保存状态等。
    Close(operator UnitIdentifier)

    // Closed 检查处理单元是否已关闭。
    // 返回 true 表示处理单元已关闭，false 表示仍在运行。
    // 该方法用于状态检查，避免对已关闭的单元进行操作。
    Closed() bool
}

// asUnitCloser 将 Unit 转换为 UnitCloser 接口。
// 如果给定的 unit 实现了 UnitCloser 接口，则返回对应的接口实例。
// 如果未实现该接口，则返回 nil。
// 该函数主要用于注册表内部的类型检查和转换。
func asUnitCloser(unit Unit) UnitCloser {
    if closer, ok := unit.(UnitCloser); ok {
        return closer
    }
    return nil
}
