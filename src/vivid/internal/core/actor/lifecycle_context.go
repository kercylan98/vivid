package actor

import "github.com/kercylan98/vivid/src/vivid/internal/core"

type LifecycleContext interface {
	// Kill 会以特定的 OnKill 消息终止此 Actor 的生命周期
	//
	// 假若 Actor 处于“存活”状态，该函数将会将其变更为“终止”中状态并停止对用户级消息的处理，同时向其所有子级 Actor 传播相同的关闭消息。
	// 在通知子级关闭后将等待所有子级 Actor 终止后销毁自身。
	//
	// 有关于具体的销毁流程，可参阅： TerminateTest
	Kill(info *OnKill)

	// TerminateTest 函数用于检查当前 Actor 是否可以被终止，它通常由自身销毁或子级 Actor 的销毁触发。
	//
	// 如果当前 Actor 处于“终止中”状态并且所有子级 Actor 都已终止，则将其状态更改为“已终止”，并且对其监听者及父级 Actor 传播被终止消息。
	TerminateTest(info *OnKilled)

	// Status 获取 Actor 的生命周期状态
	Status() uint32

	// Accident 声明事故发生
	Accident(reason core.Message)

	// HandleAccidentSnapshot 处理事故快照
	HandleAccidentSnapshot(snapshot Snapshot)
}
