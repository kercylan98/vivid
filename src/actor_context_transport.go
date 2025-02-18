package vivid

import (
	"fmt"
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"strconv"
	"time"
)

var (
	_ actorContextTransportInternal = (*actorContextTransportImpl)(nil)
)

func newActorContextTransportImpl(ctx *actorContext) *actorContextTransportImpl {
	return &actorContextTransportImpl{
		ActorContext: ctx,
	}
}

type actorContextTransportImpl struct {
	ActorContext
	envelope      Envelope                  // 当前消息
	watchHandlers map[string][]WatchHandler // 监视处理器（当 Key 存在表示正在监视目标）
	watchers      map[ActorRef]struct{}     // 该 Actor 的监视者
}

func (ctx *actorContextTransportImpl) Sender() ActorRef {
	if ctx.envelope == nil {
		return nil
	}
	return ctx.envelope.GetSender()
}

func (ctx *actorContextTransportImpl) Message() Message {
	if ctx.envelope == nil {
		return nil
	}
	return ctx.envelope.GetMessage()
}

func (ctx *actorContextTransportImpl) Reply(message Message) {
	var target = ctx.envelope.GetAgent()
	if target == nil {
		target = ctx.Sender()
	}
	ctx.tell(target, message, UserMessage)
}

func (ctx *actorContextTransportImpl) Ping(target ActorRef, timeout ...time.Duration) (pong Pong, err error) {
	return pong, ctx.ask(target, ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnPing(), SystemMessage, timeout...).Adapter(FutureAdapter[Pong](func(p Pong, err error) error {
		p = pong
		return err
	}))
}

func (ctx *actorContextTransportImpl) getWatcherHandlers(watcher ActorRef) ([]WatchHandler, bool) {
	handlers, exist := ctx.watchHandlers[watcher.String()]
	return handlers, exist
}

func (ctx *actorContextTransportImpl) deleteWatcherHandlers(watcher ActorRef) {
	delete(ctx.watchHandlers, watcher.String())
	if len(ctx.watchHandlers) == 0 {
		ctx.watchHandlers = nil
	}
}

func (ctx *actorContextTransportImpl) deleteWatcher(watcher ActorRef) {
	delete(ctx.watchers, watcher)
	if len(ctx.watchers) == 0 {
		ctx.watchers = nil
	}
}

func (ctx *actorContextTransportImpl) addWatcher(watcher ActorRef) {
	if ctx.watchers == nil {
		ctx.watchers = make(map[ActorRef]struct{})
	}
	ctx.watchers[watcher] = struct{}{}
}

func (ctx *actorContextTransportImpl) getWatchers() map[ActorRef]struct{} {
	return ctx.watchers
}

func (ctx *actorContextTransportImpl) Kill(target ActorRef, reason ...string) {
	var r string
	if len(reason) > 0 {
		r = reason[0]
	}
	ctx.tell(target, ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnKill(r, ctx.Ref(), false, false), SystemMessage)
}

func (ctx *actorContextTransportImpl) PoisonKill(target ActorRef, reason ...string) {
	var r string
	if len(reason) > 0 {
		r = reason[0]
	}
	ctx.tell(target, ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnKill(r, ctx.Ref(), true, false), UserMessage)
}

func (ctx *actorContextTransportImpl) Tell(target ActorRef, message Message) {
	ctx.tell(target, message, UserMessage)
}

func (ctx *actorContextTransportImpl) tell(target ActorRef, message Message, messageType MessageType) {
	envelope := ctx.getMessageBuilder().BuildStandardEnvelope(ctx.Ref(), target, messageType, message)

	if ctx.Ref().Equal(target) {
		// 如果目标是自己，那么通过 Send 函数来对消息进行加速
		// 这个过程可避免通过进程管理器进行查找的过程，而是直接将消息发送到自身进程中
		ctx.sendToSelfProcess(envelope)
		return
	}

	ctx.sendToProcess(envelope)
}

func (ctx *actorContextTransportImpl) Ask(target ActorRef, message Message, timeout ...time.Duration) Future[Message] {
	return ctx.ask(target, message, UserMessage, timeout...)
}

func (ctx *actorContextTransportImpl) ask(target ActorRef, message Message, messageType MessageType, timeout ...time.Duration) Future[Message] {
	var t = DefaultFutureTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}

	futureRef := ctx.Ref().Sub("future-" + string(strconv.AppendInt(nil, ctx.getNextChildGuid(), 10)))
	future := newFuture[Message](ctx.System(), futureRef, t)
	ctx.sendToProcess(ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildAgentEnvelope(futureRef, ctx.Ref(), target, messageType, message))
	return future
}

func (ctx *actorContextTransportImpl) Broadcast(message Message) {
	for _, child := range ctx.getChildren() {
		ctx.tell(child, message, UserMessage)
	}
}

func (ctx *actorContextTransportImpl) onProcessMessage(envelope Envelope) {
	ctx.envelope = envelope
	switch envelope.GetMessageType() {
	case SystemMessage:
		ctx.onProcessSystemMessage(envelope)
	case UserMessage:
		ctx.onProcessUserMessage(envelope)
	default:
		panic("unknown message type")
	}
}

func (ctx *actorContextTransportImpl) onProcessSystemMessage(envelope Envelope) {
	switch m := envelope.GetMessage().(type) {
	case OnLaunch:
		ctx.onReceive()
	case OnKill:
		ctx.onKill(m)
	case OnKilled:
		ctx.onKilled()
	case OnWatch:
		ctx.onWatch()
	case OnUnwatch:
		ctx.deleteWatcher(ctx.Sender())
	case OnWatchStopped:
		ctx.onWatchStopped(m)
	case OnPing:
		ctx.Reply(ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildPong(m))
	case AccidentRecord:
		ctx.onAccidentRecord(m)
	case *accidentFinished:
		ctx.onAccidentFinished(m.AccidentRecord)
	case contextFunc:
		m(ctx)
	case accidentTimingTask:
		m(ctx)
	default:
		panic("unknown system message")
	}
}

func (ctx *actorContextTransportImpl) onProcessUserMessage(envelope Envelope) {
	switch m := envelope.GetMessage().(type) {
	case OnWatchStopped:
		ctx.onWatchStopped(m)
	case OnKill:
		ctx.onKill(m) // 用户消息已被处理，转为终止 Actor
	case TimingTask:
		m.Execute(ctx)
	case contextFunc:
		m(ctx)
	default:
		ctx.onReceive()
	}
}
func (ctx *actorContextTransportImpl) onReceive() {
	// 交由用户处理的消息需保证异常捕获
	defer func() {
		if reason := recover(); reason != nil {
			ctx.onAccident(reason)
		}
	}()

	ctx.getActor().OnReceive(ctx)

	switch ctx.Message().(type) {
	case OnLaunch:
		ctx.removeAccidentRecord(func(record AccidentRecord) {
			ctx.Logger().Debug("actor", log.String("event", "restarted"), log.String("ref", ctx.Ref().String()))
		})
	}
}

func getActorWatchTimingLoopTaskKey(ref ActorRef) string {
	return fmt.Sprintf(timingWheelNameWatchFormater, ref.String())
}

func (ctx *actorContextTransportImpl) Watch(target ActorRef, handlers ...WatchHandler) error {
	var handler = func() error {
		watchHandlerKey := target.String()
		currHandlers, exist := ctx.watchHandlers[watchHandlerKey]
		if !exist {
			onWatch := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnWatch()
			// Actor 自身等待自身将会超时，因此需要转为 tell 消息进行处理
			if ctx.Ref().Equal(target) {
				ctx.tell(ctx.Ref(), onWatch, SystemMessage)
			} else {
				if err := ctx.ask(target, onWatch, SystemMessage).Wait(); err != nil {
					return err
				}
			}
		}

		if ctx.watchHandlers == nil {
			ctx.watchHandlers = make(map[string][]WatchHandler)
		}

		currHandlers = append(currHandlers, handlers...)
		ctx.watchHandlers[watchHandlerKey] = currHandlers

		// 通过 Ping/Pong 机制来保证监视的有效性，避免监视者已经终止但是监视者未收到通知，从而导致资源泄漏
		// 需要确保监听对象非自身
		if !target.Equal(ctx.Ref()) {
			tw := ctx.getTimingWheel()
			taskName := getActorWatchTimingLoopTaskKey(target)
			interval := time.Second * 5

			// 异步任务，严格避免操作 ctx 的行为
			tw.Loop(taskName, interval, timing.NewLoopTask(interval, -1, timing.TaskFn(func() {
				_, err := ctx.Ping(target, time.Second*3)
				if err == nil {
					return
				}
				tw.Stop(taskName)
				onWatchStopped := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnWatchStopped(target)
				ctx.tell(ctx.Ref(), onWatchStopped, UserMessage)
			})))
		}
		return nil
	}

	if ctx.Parent() == nil {
		// 对于根 Actor，需要转为消息进行处理，避免直接调用导致竞态问题
		// 因为主线程或其他 Actor 中调用会导致消息和 Watch 函数并行
		result := make(chan error)
		ctx.tell(ctx.Ref(), contextFunc(func(ctx ActorContext) {
			result <- handler()
		}), SystemMessage)
		return <-result
	}

	return handler()
}

func (ctx *actorContextTransportImpl) onWatch() {
	if ctx.terminated() {
		// 如果自身已经死亡，应该立即通知监视者
		onWatchStopped := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnWatchStopped(ctx.Ref())
		ctx.Reply(nil)
		if ctx.Sender().Equal(ctx.Ref()) {
			// 此处转为下一条消息进行处理，避免处理器还未添加就执行了
			ctx.tell(ctx.Ref(), onWatchStopped, SystemMessage)
		} else {
			ctx.tell(ctx.Sender(), onWatchStopped, UserMessage) // 通过用户消息告知已死
		}
		return
	}
	ctx.addWatcher(ctx.Sender())
	ctx.Reply(nil)
}

func (ctx *actorContextTransportImpl) onWatchStopped(m OnWatchStopped) {
	target := m.GetRef()
	ctx.getTimingWheel().Stop(getActorWatchTimingLoopTaskKey(target)) // 停止监视心跳定时器
	handlers, _ := ctx.getWatcherHandlers(target)

	if len(handlers) == 0 {
		// 未设置处理器，交由用户处理
		ctx.onReceive()
	} else {
		// 交由处理器处理
		for _, handler := range handlers {
			handler.Handle(ctx, m)
		}
	}

	// 释放处理器
	ctx.deleteWatcherHandlers(target)
}

func (ctx *actorContextTransportImpl) Unwatch(target ActorRef) {
	handler := func() {
		watchHandlerKey := target.String()
		if _, exist := ctx.watchHandlers[watchHandlerKey]; !exist {
			return
		}

		onUnwatch := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnUnwatch()
		ctx.tell(target, onUnwatch, SystemMessage)

		delete(ctx.watchHandlers, watchHandlerKey)
		if len(ctx.watchHandlers) == 0 {
			ctx.watchHandlers = nil
		}
	}

	// 对于根 Actor，需要转为消息进行处理，避免直接调用导致竞态问题
	if ctx.Parent() == nil {
		ctx.tell(ctx.Ref(), contextFunc(func(ctx ActorContext) {
			handler()
		}), SystemMessage)
		return
	}

	handler()
}

func (ctx *actorContextTransportImpl) Restart(target ActorRef, gracefully bool, reason ...string) {
	var r string
	if len(reason) > 0 {
		r = reason[0]
	}
	var messageType MessageType
	if gracefully {
		messageType = UserMessage
	} else {
		messageType = SystemMessage
	}
	ctx.tell(target, ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnKill(r, ctx.Ref(), gracefully, true), messageType)
}
