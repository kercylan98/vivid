package mailbox

type Mailbox interface {
    PushSystemMessage(message any)

    PushUserMessage(message any)

    PopSystemMessage() (message any)

    PopUserMessage() (message any)

    GetSystemMessageNum() int32

    GetUserMessageNum() int32

    Suspend()

    Resume()

    // Suspended 返回邮箱是否被挂起
    Suspended() bool
}

type Provider interface {
    Provide() Mailbox
}

type ProviderFN func() Mailbox

func (p ProviderFN) Provide() Mailbox {
    return p()
}
