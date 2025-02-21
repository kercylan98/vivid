package vivid

import "github.com/kercylan98/go-log/log"

var (
	_ actorContextPersistentInternal = (*actorContextPersistentImpl)(nil)
)

// PersistentEvent 创建一个持久化消息
func PersistentEvent(message Message) Message {
	return &persistentEvent{
		m: message,
	}
}

// persistentEvent 需要持久化的消息
type persistentEvent struct {
	m Message
}

func newActorContextPersistentImpl(ctx ActorContext) *actorContextPersistentImpl {
	impl := &actorContextPersistentImpl{
		ctx: ctx,
	}

	return impl
}

type actorContextPersistentImpl struct {
	ctx               ActorContext // ActorContext 的实例
	persistentMessage bool         // 当前消息是否持久化消息
	events            []Message    // 当前持久化事件列表
	snapshot          Message      // 当前快照
	recovering        bool         // 是否正在恢复
}

func (a *actorContextPersistentImpl) persistentRecover() {
	storage := a.ctx.getConfig().FetchPersistentStorage()
	if storage == nil {
		return
	}

	a.persistentMessage = false

	snapshot, events, err := storage.Load(a.ctx.getConfig().FetchPersistentId())
	if err != nil {
		a.ctx.Logger().Error("actor", log.String("event", "recover"), log.String("ref", a.ctx.Ref().String()), log.Err(err))
		a.ctx.onAccident(err) // 恢复失败，触发事故处理
		return
	}

	var envelope Envelope
	if snapshot != nil {
		a.recovering = true
		envelope = a.ctx.getMessageBuilder().BuildStandardEnvelope(a.ctx.Ref(), a.ctx.Ref(), UserMessage, a.snapshot)
		a.ctx.onReceiveEnvelope(envelope)
	}
	if len(a.events) > 0 {
		a.recovering = true
		if envelope == nil {
			envelope = a.ctx.getMessageBuilder().BuildStandardEnvelope(a.ctx.Ref(), a.ctx.Ref(), UserMessage, a.events)
		}
		for _, event := range a.events {
			envelope.SetMessage(event)
			a.ctx.onReceiveEnvelope(envelope)
		}
	}

	a.snapshot = snapshot
	a.events = events

	interval := a.ctx.getConfig().FetchPersistentInterval()
	if interval > 0 {
		a.ctx.ForeverLoop("persistent_recover:"+a.ctx.Ref().String(), interval, interval, TimingTaskFn(func(ctx ActorContext) {
			_ = ctx.Persist()
		}))
	}
	return
}

func (a *actorContextPersistentImpl) Snapshot(snapshot Message) {
	storage := a.ctx.getConfig().FetchPersistentStorage()
	if storage == nil {
		a.ctx.Logger().Warn("actor", log.String("event", "snapshot"), log.String("ref", a.ctx.Ref().String()), log.String("info", "storage is nil"))
		return
	}

	a.snapshot = snapshot
	a.events = nil
}

func (a *actorContextPersistentImpl) Persist() error {
	storage := a.ctx.getConfig().FetchPersistentStorage()
	if storage == nil {
		a.ctx.Logger().Warn("actor", log.String("event", "persist"), log.String("ref", a.ctx.Ref().String()), log.String("info", "storage is nil"))
		return nil
	}

	return storage.Save(a.ctx.getConfig().FetchPersistentId(), a.snapshot, a.events)
}

func (a *actorContextPersistentImpl) persistentMessageParse(envelope Envelope) Envelope {
	switch m := envelope.GetMessage().(type) {
	case *persistentEvent:
		storage := a.ctx.getConfig().FetchPersistentStorage()
		if storage == nil {
			a.ctx.Logger().Warn("actor", log.String("event", "persistent_event"), log.String("ref", a.ctx.Ref().String()), log.String("info", "storage is nil"))
			envelope.SetMessage(m)
			return envelope
		}
		a.persistentMessage = true
		a.events = append(a.events, m.m)
		envelope.SetMessage(m.m)
		return envelope
	default:
		return envelope
	}
}

func (a *actorContextPersistentImpl) isPersistentMessage() bool {
	return a.persistentMessage
}

func (a *actorContextPersistentImpl) setPersistentMessage() {
	a.persistentMessage = true
}
