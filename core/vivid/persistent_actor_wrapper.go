package vivid

import (
	"context"
	"fmt"
	"math"
	"time"
)

// persistentActorWrapper 包装持久化 Actor，提供自动恢复功能
type persistentActorWrapper struct {
	actor          PersistentActor
	store          PersistenceStore
	config         *PersistenceConfiguration
	persistenceID  string
	sequenceNumber int64
	isRecovering   bool
	recovered      bool // 标记是否已完成恢复

	// 批量处理相关
	eventBatch   []Event
	batchTimer   *time.Timer
	lastSnapshot int64 // 上次快照时的序列号
}

// newPersistentActorWrapper 创建持久化 Actor 包装器
func newPersistentActorWrapper(actor PersistentActor, store PersistenceStore, config *PersistenceConfiguration) *persistentActorWrapper {
	return &persistentActorWrapper{
		actor:         actor,
		store:         store,
		config:        config,
		persistenceID: actor.PersistenceID(),
		eventBatch:    make([]Event, 0, config.EventBatchSize),
	}
}

// Receive 实现 Actor 接口，处理消息并自动管理持久化
func (w *persistentActorWrapper) Receive(ctx ActorContext) {
	// 处理生命周期消息
	switch ctx.Message().(type) {
	case *OnLaunch:
		// Actor 启动时进行恢复
		w.recover(ctx)
		// 调用原始 Actor 处理 OnLaunch
		w.actor.Receive(ctx)
		return

	case *OnRestart:
		// Actor 重启时重新恢复
		w.recovered = false // 重置恢复标志
		w.recover(ctx)
		// 调用原始 Actor 处理 OnRestart
		w.actor.Receive(ctx)
		return

	case *OnPreRestart:
		// 先让用户处理 OnPreRestart（可能会修改状态）
		w.actor.Receive(ctx)
		// 用户处理完成后，进行关键持久化
		w.handleCriticalPersistence(ctx)
		return

	case *OnKill:
		// 先让用户处理 OnKill（可能会进行最终状态更新）
		w.actor.Receive(ctx)
		// 用户处理完成后，进行关键持久化
		w.handleCriticalPersistence(ctx)
		return
	}

	// 处理普通消息前检查是否需要刷新批量事件
	if w.shouldFlushEvents() {
		if err := w.flushEvents(ctx); err != nil {
			ctx.Logger().Error("Failed to flush events", "error", err)
		}
	}

	// 调用原始 Actor 的 Receive 方法处理普通消息
	w.actor.Receive(ctx)

	// 消息处理后检查是否需要自动快照
	w.checkAutoSnapshot(ctx)
}

// handleCriticalPersistence 处理关键生命周期的持久化
func (w *persistentActorWrapper) handleCriticalPersistence(ctx ActorContext) {
	ctx.Logger().Info("Starting critical persistence", "persistenceID", w.persistenceID)

	// 刷新待处理事件
	if err := w.flushEventsWithRetry(ctx); err != nil {
		w.handlePersistenceFailure(ctx, err)
		return
	}

	// 创建快照
	if err := w.makeSnapshotWithRetry(ctx); err != nil {
		w.handlePersistenceFailure(ctx, err)
		return
	}

	ctx.Logger().Info("Critical persistence completed", "persistenceID", w.persistenceID)
}

// flushEventsWithRetry 带重试的事件刷新
func (w *persistentActorWrapper) flushEventsWithRetry(ctx ActorContext) error {
	if len(w.eventBatch) == 0 {
		return nil
	}

	return w.retryWithBackoff(ctx, func() error {
		return w.flushEvents(ctx)
	})
}

// makeSnapshotWithRetry 带重试的快照创建
func (w *persistentActorWrapper) makeSnapshotWithRetry(ctx ActorContext) error {
	return w.retryWithBackoff(ctx, func() error {
		return w.makeSnapshot(ctx)
	})
}

// retryWithBackoff 指数退避重试
func (w *persistentActorWrapper) retryWithBackoff(ctx ActorContext, fn func() error) error {
	maxRetries := 5                     // 最大重试次数
	baseDelay := 100 * time.Millisecond // 基础延迟

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避延迟
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			ctx.Logger().Warn("Retrying critical persistence operation", "attempt", attempt, "delay", delay, "error", lastErr)

			time.Sleep(delay)
		}

		if err := fn(); err != nil {
			lastErr = err
			ctx.Logger().Error("Critical persistence operation failed", "attempt", attempt, "error", err)
			continue
		}

		// 成功
		if attempt > 0 {
			ctx.Logger().Info("Critical persistence operation succeeded after retry", "attempt", attempt)
		}
		return nil
	}

	return lastErr
}

// handlePersistenceFailure 处理持久化失败
func (w *persistentActorWrapper) handlePersistenceFailure(ctx ActorContext, err error) {
	ctx.Logger().Error("Critical persistence failed after all retries",
		"persistenceID", w.persistenceID,
		"error", err)

	// 交给用户存储层接口处理
	snapshot := newSnapshot(w.persistenceID, w.sequenceNumber, w.actor.Snapshot(), time.Now())
	w.store.OnPersistenceFailed(ctx, snapshot, w.eventBatch)
}

// recover 执行恢复流程 - 在 Actor 生命周期的正确时机执行
// 恢复失败时会触发致命错误，确保 Actor 不会在错误状态下运行
func (w *persistentActorWrapper) recover(ctx ActorContext) {
	w.isRecovering = true
	defer func() {
		w.isRecovering = false
	}()

	ctx.Logger().Info("Starting recovery", "persistenceID", w.persistenceID)

	// 加载最新快照
	snapshot, err := w.store.LoadSnapshot(context.Background(), w.persistenceID)
	if err != nil {
		ctx.Logger().Error("Failed to load snapshot", "error", err)
		// 恢复失败，触发致命错误
		panic(fmt.Errorf("persistence recovery failed: unable to load snapshot for %s: %w", w.persistenceID, err))
	}

	var fromSequenceNumber int64 = 0

	// 如果有快照，先恢复快照状态
	if snapshot != nil {
		if err := w.actor.RestoreFromSnapshot(snapshot.SnapshotData); err != nil {
			ctx.Logger().Error("Failed to restore from snapshot", "error", err)
			// 快照恢复失败，触发致命错误
			panic(fmt.Errorf("persistence recovery failed: unable to restore from snapshot for %s: %w", w.persistenceID, err))
		}
		w.sequenceNumber = snapshot.SequenceNumber
		w.lastSnapshot = snapshot.SequenceNumber
		fromSequenceNumber = snapshot.SequenceNumber + 1
		ctx.Logger().Info("Restored from snapshot",
			"persistenceID", w.persistenceID,
			"sequenceNumber", snapshot.SequenceNumber)
	}

	// 加载快照之后的所有事件
	events, err := w.store.LoadEvents(context.Background(), w.persistenceID, fromSequenceNumber)
	if err != nil {
		ctx.Logger().Error("Failed to load events", "error", err)
		// 事件加载失败，触发致命错误
		panic(fmt.Errorf("persistence recovery failed: unable to load events for %s: %w", w.persistenceID, err))
	}

	// 回放事件 - 设置消息并调用 Actor 处理
	originalMessage := ctx.Message()
	originalSender := ctx.Sender()

	for _, event := range events {
		// 临时设置事件数据为当前消息
		if actorCtx, ok := ctx.(*actorContext); ok {
			actorCtx.message = event.EventData
			actorCtx.sender = nil // 恢复期间没有发送者
		}

		// 调用 Actor 处理事件（如果处理失败会被上层的 panic 处理机制捕获）
		w.actor.Receive(ctx)
		w.sequenceNumber = event.SequenceNumber
	}

	// 恢复原始消息和发送者
	if actorCtx, ok := ctx.(*actorContext); ok {
		actorCtx.message = originalMessage
		actorCtx.sender = originalSender
	}

	// 只有恢复完全成功才标记为已恢复
	w.recovered = true
	ctx.Logger().Info("Recovery completed",
		"persistenceID", w.persistenceID,
		"sequenceNumber", w.sequenceNumber,
		"eventsReplayed", len(events))
}

// checkAutoSnapshot 检查是否需要自动创建快照
func (w *persistentActorWrapper) checkAutoSnapshot(ctx ActorContext) {
	if !w.config.EnableAutoSnapshot {
		return
	}

	// 检查是否达到快照间隔
	if w.sequenceNumber-w.lastSnapshot >= w.config.SnapshotInterval {
		if err := w.makeSnapshot(ctx); err != nil {
			ctx.Logger().Error("Auto snapshot failed", "error", err)
		}
	}
}

// makeSnapshot 创建快照
func (w *persistentActorWrapper) makeSnapshot(ctx ActorContext) error {
	snapshotData := w.actor.Snapshot()
	if snapshotData == nil {
		return nil // Actor 不需要创建快照
	}

	snapshot := newSnapshot(w.persistenceID, w.sequenceNumber, snapshotData, time.Now())

	if err := w.store.SaveSnapshot(context.Background(), snapshot); err != nil {
		return err
	}

	w.lastSnapshot = w.sequenceNumber
	ctx.Logger().Info("Snapshot created",
		"persistenceID", w.persistenceID,
		"sequenceNumber", w.sequenceNumber)
	return nil
}

// flushEvents 刷新批量事件到存储层
func (w *persistentActorWrapper) flushEvents(ctx ActorContext) error {
	if len(w.eventBatch) == 0 {
		return nil
	}

	// 批量保存事件
	for _, event := range w.eventBatch {
		if err := w.store.SaveEvent(context.Background(), event); err != nil {
			ctx.Logger().Error("Failed to save event", "error", err, "sequenceNumber", event.SequenceNumber)
			return err
		}
	}

	ctx.Logger().Debug("Events flushed",
		"persistenceID", w.persistenceID,
		"eventCount", len(w.eventBatch))

	// 清空批量缓存
	w.eventBatch = w.eventBatch[:0]

	// 重置定时器
	if w.batchTimer != nil {
		w.batchTimer.Stop()
		w.batchTimer = nil
	}

	return nil
}

// addEventToBatch 添加事件到批量缓存
func (w *persistentActorWrapper) addEventToBatch(ctx ActorContext, event Event) error {
	w.eventBatch = append(w.eventBatch, event)

	// 检查是否达到批量大小
	if len(w.eventBatch) >= w.config.EventBatchSize {
		return w.flushEvents(ctx)
	}

	// 如果还没有定时器，启动定时器
	// 注意：定时器到期时，我们不能直接调用 flushEvents，因为那时可能没有 ActorContext
	// 实际的刷新会在下次消息处理时检查并执行
	if w.batchTimer == nil {
		w.batchTimer = time.AfterFunc(w.config.EventFlushInterval, func() {
			// 定时器到期，标记需要刷新，实际刷新在下次消息处理时进行
			w.batchTimer = nil
		})
	}

	return nil
}

// shouldFlushEvents 检查是否应该刷新事件（在消息处理时调用）
func (w *persistentActorWrapper) shouldFlushEvents() bool {
	// 如果定时器已过期（batchTimer 为 nil）且有待处理事件，则需要刷新
	return w.batchTimer == nil && len(w.eventBatch) > 0
}

// persistenceContextImpl 实现 PersistenceContext 接口
type persistenceContextImpl struct {
	actorContext *actorContext
	wrapper      *persistentActorWrapper
}

func (p *persistenceContextImpl) IsRecovering() bool {
	return p.wrapper.isRecovering
}

func (p *persistenceContextImpl) LastSequenceNumber() int64 {
	return p.wrapper.sequenceNumber
}

func (p *persistenceContextImpl) PersistEvent(eventData Message, callback EventHandler) error {
	// 恢复期间不记录事件
	if p.wrapper.isRecovering {
		if callback != nil {
			callback.OnEventPersisted(Event{})
		}
		return nil
	}
	return p.persistEventInternal(eventData, callback, false)
}

func (p *persistenceContextImpl) PersistEventSync(eventData Message) error {
	// 恢复期间不记录事件
	if p.wrapper.isRecovering {
		return nil
	}
	return p.persistEventInternal(eventData, nil, true)
}

func (p *persistenceContextImpl) MakeSnapshot() error {
	// 恢复期间不保存快照
	if p.wrapper.isRecovering {
		return nil
	}

	// 先刷新所有待处理的事件
	if err := p.wrapper.flushEvents(p.actorContext); err != nil {
		return err
	}

	// 使用包装器的方法创建快照
	return p.wrapper.makeSnapshot(p.actorContext)
}

func (p *persistenceContextImpl) persistEventInternal(eventData Message, callback EventHandler, sync bool) error {
	// 生成新的序列号
	p.wrapper.sequenceNumber++

	event := Event{
		PersistenceID:  p.wrapper.persistenceID,
		SequenceNumber: p.wrapper.sequenceNumber,
		EventType:      "default",
		EventData:      eventData,
		Timestamp:      time.Now(),
	}

	if sync {
		// 同步模式：立即保存事件
		if err := p.wrapper.store.SaveEvent(context.Background(), event); err != nil {
			// 回滚序列号
			p.wrapper.sequenceNumber--
			return err
		}

		// 执行回调（用户在回调中更新状态）
		if callback != nil {
			callback.OnEventPersisted(event)
		}
	} else {
		// 异步模式：添加到批量缓存
		if err := p.wrapper.addEventToBatch(p.actorContext, event); err != nil {
			// 回滚序列号
			p.wrapper.sequenceNumber--
			return err
		}

		// 执行回调（用户在回调中更新状态）
		if callback != nil {
			callback.OnEventPersisted(event)
		}
	}

	return nil
}
