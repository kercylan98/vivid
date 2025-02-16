package vivid

import (
	"fmt"
	"github.com/kercylan98/chrono/timing"
	"strconv"
	"time"
)

const (
	timingWheelNameWatchFormater = "[watch]%s"
)

var (
	_ actorContextActionsInternal = (*actorContextActionsImpl)(nil)
)

func newActorContextActionsImpl(ctx *actorContext) actorContextActionsInternal {
	return &actorContextActionsImpl{
		ActorContext: ctx,
	}
}

type actorContextActionsImpl struct {
	ActorContext
	watchHandlers map[ActorRef][]WatchHandler // 监视处理器（当 Key 存在表示正在监视目标）
	watchers      map[ActorRef]struct{}       // 该 Actor 的监视者
}

func (ctx *actorContextActionsImpl) getWatcherHandlers(watcher ActorRef) ([]WatchHandler, bool) {
	handlers, exist := ctx.watchHandlers[watcher]
	return handlers, exist
}

func (ctx *actorContextActionsImpl) deleteWatcherHandlers(watcher ActorRef) {
	delete(ctx.watchHandlers, watcher)
	if len(ctx.watchHandlers) == 0 {
		ctx.watchHandlers = nil
	}
}

func (ctx *actorContextActionsImpl) deleteWatcher(watcher ActorRef) {
	delete(ctx.watchers, watcher)
	if len(ctx.watchers) == 0 {
		ctx.watchers = nil
	}
}

func (ctx *actorContextActionsImpl) addWatcher(watcher ActorRef) {
	if ctx.watchers == nil {
		ctx.watchers = make(map[ActorRef]struct{})
	}
	ctx.watchers[watcher] = struct{}{}
}

func (ctx *actorContextActionsImpl) getWatchers() map[ActorRef]struct{} {
	return ctx.watchers
}

func (ctx *actorContextActionsImpl) Kill(target ActorRef, reason ...string) {
	var r string
	if len(reason) > 0 {
		r = reason[0]
	}
	ctx.tell(target, ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnKill(r, ctx.Ref(), false), SystemMessage)
}

func (ctx *actorContextActionsImpl) PoisonKill(target ActorRef, reason ...string) {
	var r string
	if len(reason) > 0 {
		r = reason[0]
	}
	ctx.tell(target, ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnKill(r, ctx.Ref(), true), UserMessage)
}

func (ctx *actorContextActionsImpl) Tell(target ActorRef, message Message) {
	ctx.tell(target, message, UserMessage)
}

func (ctx *actorContextActionsImpl) tell(target ActorRef, message Message, messageType MessageType) {
	envelope := ctx.getMessageBuilder().BuildStandardEnvelope(ctx.Ref(), target, messageType, message)

	if ctx.Ref().Equal(target) {
		// 如果目标是自己，那么通过 Send 函数来对消息进行加速
		// 这个过程可避免通过进程管理器进行查找的过程，而是直接将消息发送到自身进程中
		ctx.sendToSelfProcess(envelope)
		return
	}

	ctx.sendToProcess(envelope)
}

func (ctx *actorContextActionsImpl) Ask(target ActorRef, message Message, timeout ...time.Duration) Future[Message] {
	return ctx.ask(target, message, UserMessage, timeout...)
}

func (ctx *actorContextActionsImpl) ask(target ActorRef, message Message, messageType MessageType, timeout ...time.Duration) Future[Message] {
	var t = DefaultFutureTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}

	futureRef := ctx.Ref().Sub("future-" + string(strconv.AppendInt(nil, ctx.getNextChildGuid(), 10)))
	future := newFuture[Message](ctx.System(), futureRef, t)
	ctx.sendToProcess(ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildAgentEnvelope(futureRef, ctx.Ref(), target, messageType, message))
	return future
}

func getActorWatchTimingLoopTaskKey(ref ActorRef) string {
	return fmt.Sprintf(timingWheelNameWatchFormater, ref.String())
}

func (ctx *actorContextActionsImpl) Watch(target ActorRef, handlers ...WatchHandler) error {
	currHandlers, exist := ctx.watchHandlers[target]
	if !exist {
		onWatch := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnWatch()
		if err := ctx.ask(target, onWatch, SystemMessage).Wait(); err != nil {
			return err
		}
	}

	if ctx.watchHandlers == nil {
		ctx.watchHandlers = make(map[ActorRef][]WatchHandler)
	}

	currHandlers = append(currHandlers, handlers...)
	ctx.watchHandlers[target] = currHandlers

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

func (ctx *actorContextActionsImpl) Unwatch(target ActorRef) {
	if _, exist := ctx.watchHandlers[target]; !exist {
		return
	}

	onUnwatch := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnUnwatch()
	ctx.tell(target, onUnwatch, SystemMessage)

	delete(ctx.watchHandlers, target)
	if len(ctx.watchHandlers) == 0 {
		ctx.watchHandlers = nil
	}
}
