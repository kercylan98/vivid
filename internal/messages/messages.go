package messages

import (
	"fmt"
	"time"
)

type (
	// Command 表示命令消息的命令
	Command uint8
)

const (
	CommandPauseMailbox  Command = iota // 暂停邮箱消息处理
	CommandResumeMailbox                // 恢复邮箱消息处理
)

func (c Command) String() string {
	var name string
	switch c {
	case CommandPauseMailbox:
		name = "CommandPauseMailbox"
	case CommandResumeMailbox:
		name = "CommandResumeMailbox"
	default:
		name = "UnknownCommand"
	}
	return fmt.Sprintf("%s(%d)", name, uint8(c))
}

func (c Command) Build() *NoneArgsCommandMessage {
	return &NoneArgsCommandMessage{Command: c}
}

// NoneArgsCommandMessage 表示没有参数的命令消息，行为由枚举决定
type NoneArgsCommandMessage struct {
	Command Command
}

type PingMessage struct {
	Time time.Time // 发出 Ping 消息的时间
}

type PongMessage struct {
	Ping        *PingMessage // 对应的 Ping 消息
	RespondTime time.Time    // 响应 Pong 消息的时间
}

type WatchMessage struct{}

type UnwatchMessage struct {
}
