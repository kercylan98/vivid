package vivid

import "github.com/kercylan98/wasteland/src/wasteland"

func newActorContextProcess(actorContextBasic actorContextBasic) actorContextProcess {
	return &actorContextProcessImpl{
		actorContextBasic: actorContextBasic,
	}
}

type actorContextProcess interface {
	wasteland.Process
	wasteland.ProcessLifecycle
	wasteland.ProcessHandler
}

type actorContextProcessImpl struct {
	actorContextBasic actorContextBasic
}

func (a *actorContextProcessImpl) GetID() wasteland.ProcessId {
	return a.actorContextBasic.Ref().(actorRefProcessInfo).processId()
}

func (a *actorContextProcessImpl) Initialize() {
	//TODO implement me
	panic("implement me")
}

func (a *actorContextProcessImpl) HandleMessage(sender wasteland.ProcessId, priority wasteland.MessagePriority, message wasteland.Message) {
	//TODO implement me
	panic("implement me")
}

func (a *actorContextProcessImpl) Terminate(operator wasteland.ProcessId) {
	//TODO implement me
	panic("implement me")
}

func (a *actorContextProcessImpl) Terminated() bool {
	//TODO implement me
	panic("implement me")
}
