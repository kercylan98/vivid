package actor

type LifecycleContext interface {
	Kill(info *OnKill)

	TryRefreshTerminateStatus(info *OnKilled)

	// Status 获取 Actor 状态
	Status() uint32
}
