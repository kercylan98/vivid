package actor

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

type virtualKey struct {
	kind string
	name string
}

type virtualDelivery struct {
	system    bool
	sender    vivid.ActorRef
	recipient vivid.ActorRef
	message   vivid.Message
	request   *future.Future[vivid.Message] // Tell 不持有请求；已结束的排队 Ask 不再投递
}

type virtualActivation struct {
	key      virtualKey
	provider vivid.ActorProvider
	mailbox  *virtualMailbox
	queue    []virtualDelivery
	creating bool
}

// virtualCoordinator 只协调本系统的逻辑身份与物理实例，不保存业务 ActorContext 供外部调用。
// 锁内只更新登记和默认邮箱，Provider、生命周期及失败回调均在锁外执行。
type virtualCoordinator struct {
	system  *System
	options vivid.VirtualActorOptions
	mu      sync.Mutex
	entries map[virtualKey]*virtualActivation
	waiting []*virtualActivation
	running int
	pending int
	closed  bool
}

func newVirtualCoordinator(system *System) *virtualCoordinator {
	options := system.options.VirtualActorOptions
	if options.IdleTimeout == 0 {
		options.IdleTimeout = 10 * time.Minute
	}
	if options.ActivationConcurrency == 0 {
		options.ActivationConcurrency = 16
	}
	if options.PendingMessageLimit == 0 {
		options.PendingMessageLimit = 4096
	}
	return &virtualCoordinator{system: system, options: options, entries: make(map[virtualKey]*virtualActivation)}
}

func (s *System) dispatchVirtual(system bool, sender, recipient vivid.ActorRef, message vivid.Message, request *future.Future[vivid.Message]) error {
	if s.virtualCoordinator == nil {
		return vivid.ErrorVirtualActorProviderNotFound
	}
	return s.virtualCoordinator.dispatch(virtualDelivery{system: system, sender: sender, recipient: recipient, message: message, request: request})
}

func (v *virtualCoordinator) dispatch(delivery virtualDelivery) error {
	identity, ok := delivery.recipient.(*virtual.Identity)
	if !ok || identity == nil {
		return vivid.ErrorVirtualRecipientException
	}
	key := virtualKey{kind: identity.Kind(), name: identity.Name()}
	if key.kind == "" || key.name == "" {
		return vivid.ErrorIllegalArgument.WithMessage("virtual actor kind and name are required")
	}
	provider := v.system.options.VirtualActorProviders[key.kind]
	if provider == nil {
		return vivid.ErrorVirtualActorProviderNotFound.WithMessage(key.kind)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return vivid.ErrorActorSystemStopped
	}
	if delivery.request != nil && delivery.request.IsCompleted() {
		return nil
	}
	entry := v.entries[key]
	if entry != nil && !entry.creating && entry.mailbox != nil && !entry.mailbox.closing {
		entry.mailbox.deliverLocked(delivery)
		return nil
	}
	if v.pending >= v.options.PendingMessageLimit {
		return vivid.ErrorVirtualPendingMessagesFull
	}
	if entry == nil {
		entry = &virtualActivation{key: key, provider: vivid.ActorProviderFN(func() vivid.Actor {
			return provider.Provide(key.kind, key.name)
		})}
		v.entries[key] = entry
		v.waiting = append(v.waiting, entry)
	}
	entry.queue = append(entry.queue, delivery)
	v.pending++
	v.startLocked()
	return nil
}

func (v *virtualCoordinator) startLocked() {
	for !v.closed && v.running < v.options.ActivationConcurrency && len(v.waiting) > 0 {
		parent, err := v.system.beginActorCreation()
		if err != nil {
			// Stop 关闭创建准入后会清空尚未开始的等待，不在锁内结束 Future。
			return
		}
		entry := v.waiting[0]
		v.waiting[0] = nil
		v.waiting = v.waiting[1:]
		entry.creating = true
		v.running++
		go v.create(parent, entry)
	}
}

func (v *virtualCoordinator) create(parent *Context, entry *virtualActivation) {
	var err error
	defer v.system.actorCreations.Done()
	defer func() {
		if fault := recover(); fault != nil {
			err = vivid.ErrorActorSpawnFailed.With(fmt.Errorf("virtual actor initialization panic: %v", fault))
		}
		v.complete(entry, err)
	}()
	actor := entry.provider.Provide()
	if actor == nil {
		err = vivid.ErrorActorSpawnFailed.WithMessage("virtual actor provider returned nil")
		return
	}
	value := reflect.ValueOf(actor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			err = vivid.ErrorActorSpawnFailed.WithMessage("virtual actor provider returned nil")
			return
		}
	}
	_, err = parent.actorOf(actor, func(initializer *contextInitializer) {
		// 业务固定选项仍生效；物理名称与重启 Provider 由虚拟激活统一管理。
		initializer.options = append(initializer.options,
			vivid.WithActorName(fmt.Sprintf("@virtual-%d", actorIncrementId.Add(1))),
			vivid.WithActorProvider(entry.provider),
		)
		initializer.mailboxFactory = func(ctx *Context) vivid.Mailbox {
			m := &virtualMailbox{coordinator: v, entry: entry, ctx: ctx, lastActivity: time.Now()}
			m.Mailbox = mailbox.NewUnboundedMailbox(256, m)
			v.mu.Lock()
			entry.mailbox = m
			v.mu.Unlock()
			return m
		}
	})
}

func (v *virtualCoordinator) complete(entry *virtualActivation, err error) {
	v.mu.Lock()
	v.running--
	entry.creating = false
	queued := entry.queue
	entry.queue = nil
	v.pending -= len(queued)
	if v.closed {
		err = vivid.ErrorActorSystemStopped
	}
	if err == nil && (entry.mailbox == nil || entry.mailbox.released) {
		err = vivid.ErrorActorDeaded
	}
	if err == nil {
		for _, delivery := range queued {
			if delivery.request == nil || !delivery.request.IsCompleted() {
				entry.mailbox.deliverLocked(delivery)
			}
		}
	} else {
		delete(v.entries, entry.key)
	}
	v.startLocked()
	v.mu.Unlock()
	if err != nil {
		for _, delivery := range queued {
			v.fail(delivery, err)
		}
	}
}

func (v *virtualCoordinator) fail(delivery virtualDelivery, err error) {
	if delivery.request != nil {
		delivery.request.Close(err)
		return
	}
	v.system.Logger().Error("virtual actor delivery failed", log.Any("recipient", delivery.recipient), log.Any("error", err))
	v.system.TellSelf(ves.DeathLetterEvent{Envelope: mailbox.NewEnvelop(delivery.system, delivery.sender, delivery.recipient, delivery.message), Time: time.Now()})
}

func (v *virtualCoordinator) close() {
	v.mu.Lock()
	v.closed = true
	var queued []virtualDelivery
	for key, entry := range v.entries {
		queued = append(queued, entry.queue...)
		v.pending -= len(entry.queue)
		entry.queue = nil
		if !entry.creating && entry.mailbox == nil {
			delete(v.entries, key)
		}
	}
	v.waiting = nil
	v.mu.Unlock()
	for _, delivery := range queued {
		v.fail(delivery, vivid.ErrorActorSystemStopped)
	}
}

type virtualIdleCheck struct{ marker byte }

// virtualMailbox 保留原始信封和业务 Actor，仅在虚拟实例上增加空闲观察与投递准入。
// 协调器锁保护入队、待处理计数和钝化切换；业务执行不持有该锁。
type virtualMailbox struct {
	vivid.Mailbox
	coordinator  *virtualCoordinator
	entry        *virtualActivation
	ctx          *Context
	pending      int
	lastActivity time.Time
	closing      bool
	released     bool
	timer        *scheduler.Timer
	check        *virtualIdleCheck
}

func (m *virtualMailbox) deliverLocked(delivery virtualDelivery) {
	m.enqueueLocked(mailbox.NewEnvelop(delivery.system, delivery.sender, m.ctx.ref, delivery.message))
}

func (m *virtualMailbox) enqueueLocked(envelope vivid.Envelop) {
	if !envelope.System() {
		m.pending++
		m.lastActivity = time.Now()
	}
	m.Mailbox.Enqueue(envelope)
}

func (m *virtualMailbox) Enqueue(envelope vivid.Envelop) {
	// 自定义信封的方法在准入锁外读取，邮箱内只保留不可变快照。
	if _, ok := envelope.(*mailbox.Envelop); !ok {
		envelope = mailbox.NewEnvelop(envelope.System(), envelope.Sender(), envelope.Receiver(), envelope.Message())
	}
	v := m.coordinator
	v.mu.Lock()
	if m.closing && !envelope.System() {
		v.mu.Unlock()
		v.system.TellSelf(ves.DeathLetterEvent{Envelope: envelope, Time: time.Now()})
		return
	}
	m.enqueueLocked(envelope)
	v.mu.Unlock()
}

func (m *virtualMailbox) HandleEnvelop(envelope vivid.Envelop) {
	if check, ok := envelope.Message().(*virtualIdleCheck); ok {
		m.onIdle(check)
		return
	}
	if _, killing := envelope.Message().(*vivid.OnKill); killing {
		// 显式终止也在执行用户收尾前关闭逻辑投递，原地重启由 Context 内部推进。
		m.coordinator.mu.Lock()
		m.closing = true
		m.coordinator.mu.Unlock()
	}
	m.ctx.HandleEnvelop(envelope)
	v := m.coordinator
	v.mu.Lock()
	if !envelope.System() {
		m.pending--
		m.lastActivity = time.Now()
	}
	if _, launch := envelope.Message().(*vivid.OnLaunch); launch && !m.closing && !v.closed {
		m.scheduleLocked(v.options.IdleTimeout)
	}
	v.mu.Unlock()
}

func (m *virtualMailbox) scheduleLocked(delay time.Duration) {
	if m.timer != nil {
		m.timer.Stop()
	}
	if delay <= 0 {
		return
	}
	check := new(virtualIdleCheck)
	m.check = check
	m.timer = m.coordinator.system.timers.AfterFunc(delay, func() {
		// 系统队列保证暂停普通邮箱时仍能检查；不经过用户调度登记。
		m.Mailbox.Enqueue(mailbox.NewEnvelop(true, m.ctx.ref, m.ctx.ref, check))
	})
}

func (m *virtualMailbox) onIdle(check *virtualIdleCheck) {
	v := m.coordinator
	v.mu.Lock()
	if check != m.check || m.closing || v.closed {
		v.mu.Unlock()
		return
	}
	remaining := v.options.IdleTimeout - time.Since(m.lastActivity)
	if remaining > 0 || m.pending > 0 || m.ctx.StashCount() > 0 || atomic.LoadInt32(&m.ctx.state) != running {
		if remaining <= 0 {
			remaining = v.options.IdleTimeout
		}
		m.scheduleLocked(remaining)
		v.mu.Unlock()
		return
	}
	// 与所有入队共用准入边界；后续逻辑投递等待本实例最终清理，物理引用不迁移。
	m.closing = true
	v.mu.Unlock()
	m.ctx.Kill(m.ctx.ref, false, "virtual actor idle")
}

func (m *virtualMailbox) release() {
	v := m.coordinator
	v.mu.Lock()
	defer v.mu.Unlock()
	if m.released {
		return
	}
	m.closing, m.released = true, true
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	m.check = nil
	if v.entries[m.entry.key] != m.entry || m.entry.mailbox != m || m.entry.creating {
		return
	}
	m.entry.mailbox = nil
	if len(m.entry.queue) > 0 && !v.closed {
		v.waiting = append(v.waiting, m.entry)
		v.startLocked()
	} else {
		delete(v.entries, m.entry.key)
	}
}
