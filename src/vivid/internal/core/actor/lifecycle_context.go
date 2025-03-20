package actor

type LifecycleContext interface {
	Kill(kill *OnKill)

	TryRefreshTerminateStatus()
}
