package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/chain"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
)

const (
	ready int32 = iota
	starting
	start
	stopping
	stop
)

var (
	_ vivid.ActorSystem                 = (*System)(nil)
	_ vivid.EnvelopHandler              = (*System)(nil)
	_ remoting.ActorSystemEnvelopTarget = (*System)(nil)
	_ vivid.SystemStateProvider         = (*System)(nil)
	_ vivid.MetricsProvider             = (*System)(nil)
)

var (
	heartbeatMessage = &vivid.Heartbeat{}
)

func NewSystem(options ...vivid.ActorSystemOption) *System {
	opts := vivid.NewActorSystemOptions(options...)
	opts.Logger = log.Protect(opts.Logger)

	system := &System{
		options:              opts,
		actorContexts:        sync.Map{},
		futureAgents:         make(map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef),
		rootCleanupCompleted: make(chan struct{}),
		stopCompleted:        make(chan struct{}),
		startCompleted:       make(chan struct{}),
		timers:               scheduler.NewTimers(),
		eventStreamScope:     new(eventStreamScope),
	}

	system.options.Context, system.cancel = context.WithCancel(system.options.Context)

	system.eventStream = newEventStream(system)
	system.remotingEnvelopHandler = remoting.NewActorSystemEnvelopHandler(system, system)
	// 在任何 Actor 回调启动前固定协调器引用，启动期间不再并发替换路由入口。
	system.virtualCoordinator = newVirtualCoordinator(system)
	return system
}

type System struct {
	*Context                                                                 // ActorSystem 本身就表示了根 Actor
	codec                  *serialization.VividCodec                         // 内部消息编解码器
	actorCreations         sync.WaitGroup                                    // 状态锁内接纳创建，关闭准入后等待全部创建及回调返回
	rootChildrenLock       sync.Mutex                                        // 根子节点表由外部创建与根邮箱共享，不在回调或投递期间持有
	actorInitLock          sync.Mutex                                        // 保护创建中的名称，不在用户初始化回调期间持有
	initializingActors     map[vivid.ActorPath]*Context                      // 仅保留正在初始化的 Actor
	options                *vivid.ActorSystemOptions                         // 系统选项
	actorContexts          sync.Map                                          // 用于加速访问的 ActorContext 缓存（含有 Future）
	futureAgents           map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef // 记录本地 Actor 其正在被代理的 Future
	futureLock             sync.RWMutex                                      // Future 的锁，保证 Future 的并发安全
	requestsClosed         bool                                              // 根 Actor 收尾后关闭请求登记，由 futureLock 保护
	rootCleanupCompleted   chan struct{}                                     // 根 Actor 的框架清理已完成
	stopCompleted          chan struct{}                                     // 有序关闭及系统资源清理均已完成
	startCompleted         chan struct{}                                     // 首次启动初始化链已结束
	startOnce              sync.Once                                         // 确保初始化链结束信号只关闭一次
	remotingRef            vivid.ActorRef                                    // 远程服务器引用
	remotingMailbox        vivid.Mailbox                                     // 所有远程地址共享的发送入口
	remotingAdmission      *remoting.OutboundAdmission                       // 远程信封在进入邮箱前共享的准入名额
	remotingEnvelopHandler remoting.NetworkEnvelopHandler                    // 远程信封处理器
	eventStream            vivid.EventStream                                 // 事件流
	eventStreamScope       *eventStreamScope                                 // 本地订阅身份，不保留系统引用
	metrics                metrics.Metrics                                   // 指标收集器
	timers                 *scheduler.Timers                                 // 本系统共享的分片计时队列
	status                 int32                                             // 系统状态
	statusLock             sync.Mutex                                        // 系统状态锁
	cancel                 context.CancelFunc                                // 上下文停止函数
	startTime              time.Time                                         // 启动时间，Start() 时记录
	gossipRef              vivid.ActorRef                                    // 集群 gossip 引用
	clusterReadiness       *gossip.Readiness                                 // 本次启动的初始加入结果
	virtualCoordinator     *virtualCoordinator                               // 虚拟协调器
	closingActors          sync.Map                                          // 即将关闭的本地实例引用，不按可复用路径索引
}

func (s *System) markActorAsClosing(ref vivid.ActorRef) {
	if ref.GetAddress() != s.Ref().GetAddress() {
		return
	}
	local := ref.(*Ref)
	value, exists := s.actorContexts.Load(local.path)
	ctx, ok := value.(*Context)
	if !exists || !ok {
		return
	}
	// 原实例引用及已缓存的邮箱不能为同路径新实例登记关闭。
	if local.events != nil && local.events != ctx.ref.(*Ref).events {
		return
	}
	if cached := local.cache.Load(); cached != nil && cached.mailbox != ctx.mailbox {
		return
	}
	s.closingActors.Store(ctx.ref, struct{}{})
	// 移除路由后可能已有同名新实例，撤销旧实例的迟到登记。
	if current, exists := s.actorContexts.Load(local.path); !exists || current != ctx {
		s.closingActors.Delete(ctx.ref)
	}
}

func (s *System) unmarkActorAsClosing(ref vivid.ActorRef) {
	if ref.GetAddress() != s.Ref().GetAddress() {
		return
	}
	s.closingActors.Delete(ref.(*Ref))
}

func (s *System) HeartbeatProbe(ref vivid.ActorRef) *vivid.Heartbeat {
	heartbeat := &vivid.Heartbeat{
		Ref: ref,
	}

	// 是否存在
	value, exists := s.actorContexts.Load(ref.GetPath())
	if !exists {
		return heartbeat
	}
	// 终止或重启初始化后保留的上下文不能作为可用依据；僵尸也保持 killed 状态。
	if ctx, ok := value.(*Context); ok && atomic.LoadInt32(&ctx.state) == killed {
		return heartbeat
	}

	// 是否即将被关闭
	var closing bool
	if ctx, ok := value.(*Context); ok {
		_, closing = s.closingActors.Load(ctx.ref)
	}
	if closing {
		// 如果是上下文，还需检查是否处于多阶段停止中
		ctx, ok := value.(*Context)
		if ok {
			// 是否处于多阶段停止中，还可收到消息
			if ctx.phaseKill != nil && !ctx.phaseKill.completed {
				heartbeat.Available = true
				return heartbeat
			}

			// 状态是否是 running
			if atomic.LoadInt32(&ctx.state) == running {
				heartbeat.Available = true
				return heartbeat
			}
		}

		return heartbeat
	}

	heartbeat.Available = true
	return heartbeat
}

func (s *System) Probe(ref vivid.ActorRef, timeout ...time.Duration) vivid.Future[*vivid.Heartbeat] {
	if s.requestsStopped() {
		return future.NewFutureFail[*vivid.Heartbeat](vivid.ErrorActorSystemStopped)
	}
	if ref.GetAddress() != s.Ref().GetAddress() {
		return vivid.NewTypedFuture[vivid.Message, *vivid.Heartbeat](s.Ask(ref, heartbeatMessage, timeout...))
	}

	f := future.NewFuture[*vivid.Heartbeat](s, 0, nil)
	f.EnqueueMessage(s.HeartbeatProbe(ref))
	return f
}

func (s *System) GetSystemBasicState() vivid.SystemBasicState {
	remotingEnabled := s.options.RemotingBindAddress != ""
	addr := ""
	if remotingEnabled {
		addr = s.options.RemotingAdvertiseAddress
		if addr == "" {
			addr = s.options.RemotingBindAddress
		}
	}
	return vivid.SystemBasicState{
		StartTime:       s.startTime,
		Version:         vivid.Version,
		RemotingEnabled: remotingEnabled,
		RemotingAddress: addr,
		MetricsEnabled:  s.options.EnableMetrics,
	}
}

func (s *System) HandleFailedRemotingEnvelop(envelop vivid.Envelop) {
	// 将消息投递到死信队列
	s.TellSelf(ves.DeathLetterEvent{
		Envelope: envelop,
		Time:     time.Now(),
	})
}

func (s *System) Logger() log.Logger {
	return s.options.Logger
}

func (s *System) ResolveMailbox(receiver vivid.ActorRef) vivid.Mailbox {
	ref, ok := receiver.(*Ref)
	if !ok {
		return s.Mailbox()
	}
	return s.findMailbox(ref)
}

func (s *System) ActorOf(actor vivid.Actor, options ...vivid.ActorOption) (vivid.ActorRef, error) {
	ctx, err := s.beginActorCreation()
	if err != nil {
		return nil, err
	}
	defer s.actorCreations.Done()
	return ctx.ActorOf(actor, options...)
}

// beginActorCreation 在停止等待开始前登记创建，登记覆盖 Provider 与初始化回调。
func (s *System) beginActorCreation() (*Context, error) {
	s.statusLock.Lock()
	if s.status == stopping || s.status == stop {
		s.statusLock.Unlock()
		return nil, vivid.ErrorActorSystemStopped
	}
	ctx := s.Context
	if ctx == nil {
		s.statusLock.Unlock()
		return nil, vivid.ErrorActorSystemNotStarted
	}
	// 与停止状态转换共用准入边界，关闭等待开始后不会再增加在途创建。
	s.actorCreations.Add(1)
	s.statusLock.Unlock()
	return ctx, nil
}

func (s *System) Start() error {
	stateError, stateMessage := func(s *System) (error, string) {
		s.statusLock.Lock()
		defer s.statusLock.Unlock()
		switch s.status {
		case starting:
			return vivid.ErrorActorSystemAlreadyStarted, "actor system is starting"
		case start:
			return vivid.ErrorActorSystemAlreadyStarted, "actor system already started"
		case stopping, stop:
			return vivid.ErrorActorSystemAlreadyStopped, "actor system already stopped"
		default:
			s.status = starting
			return nil, ""
		}
	}(s)
	if stateError != nil {
		// 日志器可重入系统，解锁后记录本次判定，不再读取后续状态。
		s.Logger().Warn(stateMessage)
		return stateError
	}
	defer s.completeStart()

	s.startTime = time.Now()
	if s.options.RemotingBindAddress != "" || s.options.RemotingAdvertiseAddress != "" {
		address, ok := utils.NormalizeRemoteAddress(s.options.RemotingAdvertiseAddress)
		if !ok || s.options.RemotingBindAddress == "" {
			err := vivid.ErrorIllegalArgument.WithMessage("remoting requires a bind address and an advertise address with a specific host and port: " + s.options.RemotingAdvertiseAddress)
			s.completeStart()
			return vivid.ErrorActorSystemStartFailed.With(err).With(s.Stop(s.options.StopTimeout))
		}
		s.options.RemotingAdvertiseAddress = address
	}
	if s.options.ClusterOptions != nil {
		if err := s.options.ClusterOptions.Validate(); err != nil {
			s.completeStart()
			return vivid.ErrorActorSystemStartFailed.With(err).With(s.Stop(s.options.StopTimeout))
		}
	}
	s.Logger().Debug("actor system starting")

	availableSignal := make(chan struct{})
	systemChains := newSystemChains(availableSignal)
	startErr := func() error {
		// 回滚前解除组件的启动等待，避免停止流程等待尚未结束的 Start。
		defer close(availableSignal)
		return chain.New(chain.WithContext(s.options.Context)).
			Append(systemChains.initializeCodec(s)).
			Append(systemChains.spawnGuardActor(s)).
			Append(systemChains.initializeMetrics(s)).
			Append(systemChains.initializeRemoting(s)).
			Append(systemChains.initializeCluster(s)).
			Append(systemChains.validateVirtualActors(s)).
			Run()
	}()

	if startErr != nil {
		s.Logger().Error("actor system start failed", log.Any("err", startErr))
		s.statusLock.Lock()
		stopWon := s.status == stopping || s.status == stop
		s.statusLock.Unlock()
		s.completeStart()
		stopErr := s.Stop(s.options.StopTimeout)
		if stopWon {
			return vivid.ErrorActorSystemStopped.With(stopErr)
		}
		return vivid.ErrorActorSystemStartFailed.With(startErr).With(stopErr)
	}

	// 守护系统上下文
	go func() {
		<-s.options.Context.Done()
		_ = s.stop(false) // 无意义错误
	}()

	s.statusLock.Lock()
	if s.status != starting {
		s.statusLock.Unlock()
		s.completeStart()
		return vivid.ErrorActorSystemStopped
	}
	s.status = start
	s.statusLock.Unlock()
	s.completeStart()
	s.Logger().Debug("actor system started")
	return nil
}

func (s *System) completeStart() {
	s.startOnce.Do(func() { close(s.startCompleted) })
}
func (s *System) Stop(timeout ...time.Duration) error {
	return s.stop(true, timeout...)
}

func (s *System) stop(checkLog bool, timeout ...time.Duration) error {
	// 只有首个调用启动关闭，其余调用共享完成信号并使用各自的等待预算。
	stateError, stateMessage := func(s *System) (error, string) {
		s.statusLock.Lock()
		defer s.statusLock.Unlock()
		switch s.status {
		case ready:
			return vivid.ErrorActorSystemNotStarted, "actor system not started"
		case stop:
			return vivid.ErrorActorSystemAlreadyStopped, "actor system already stopped"
		case stopping:
			return nil, ""
		default:
			s.status = stopping
			go s.finishStop()
			return nil, ""
		}
	}(s)
	if stateError != nil {
		if checkLog {
			s.Logger().Warn(stateMessage)
		}
		return stateError
	}

	var stopTimeout = sugar.Max(sugar.FirstOrDefault(timeout, s.options.StopTimeout), 0)
	var timeoutSignal <-chan time.Time
	if stopTimeout > 0 {
		timer := time.NewTimer(stopTimeout)
		defer timer.Stop()
		timeoutSignal = timer.C
	}
	select {
	case <-s.stopCompleted:
		return nil
	case <-timeoutSignal:
		// 完成与超时同时可读时，优先报告已经完成的关闭。
		select {
		case <-s.stopCompleted:
			return nil
		default:
		}
		if checkLog {
			s.Logger().Error("actor system stop wait failed", log.Duration("timeout", stopTimeout))
		}
		return vivid.ErrorActorSystemStopFailed.With(context.DeadlineExceeded)
	}
}

func (s *System) finishStop() {
	<-s.startCompleted
	s.statusLock.Lock()
	readiness := s.clusterReadiness
	s.statusLock.Unlock()
	if readiness != nil {
		readiness.Stop()
	}
	s.Logger().Debug("actor system stopping")

	if s.virtualCoordinator != nil {
		s.virtualCoordinator.close()
	}

	// 等待已开始的外部创建完成后再通知根 Actor，避免遗漏刚创建的子节点。
	// 协调发生在独立的关闭任务中，调用方仍按自身预算超时返回。
	s.actorCreations.Wait()
	if s.Context != nil {
		s.tell(true, s.ref, &guard.Stop{})
	}

	if s.Context != nil {
		<-s.rootCleanupCompleted
	}
	if s.virtualCoordinator != nil && s.virtualCoordinator.gate != nil {
		<-s.virtualCoordinator.gate.done
	}
	s.closeFutureRequests()
	if s.remotingAdmission != nil {
		s.remotingAdmission.Stop()
	}
	s.cancel()
	s.timers.Close()

	s.statusLock.Lock()
	s.status = stop
	close(s.stopCompleted)
	s.statusLock.Unlock()
	s.Logger().Debug("actor system stopped")
}

func (s *System) requestsStopped() bool {
	select {
	case <-s.stopCompleted:
		return true
	default:
		return false
	}
}

func (s *System) appendFuture(agentRef *AgentRef, future *future.Future[vivid.Message], owner *Context, generation uint64) error {
	s.actorContexts.Store(agentRef.ref.GetPath(), future)

	s.futureLock.Lock()
	defer s.futureLock.Unlock()

	if s.requestsClosed {
		s.actorContexts.Delete(agentRef.ref.GetPath())
		return vivid.ErrorActorSystemStopped
	}
	if owner != nil && (generation&1 != 0 || owner.requestEpoch.Load() != generation) {
		s.actorContexts.Delete(agentRef.ref.GetPath())
		return vivid.ErrorActorDeaded
	}
	// 完成回调可能先清理再遇到路由注册，锁内复核结果并撤销迟到登记。
	if future.IsCompleted() {
		s.actorContexts.Delete(agentRef.ref.GetPath())
		return nil
	}
	agentPath := agentRef.agent.GetPath()
	futureMap, exists := s.futureAgents[agentPath]
	if !exists {
		futureMap = make(map[vivid.ActorPath]*AgentRef)
		s.futureAgents[agentPath] = futureMap
	}
	futureMap[agentRef.ref.GetPath()] = agentRef
	return nil
}

func (s *System) closeContextRequests(ctx *Context) {
	s.futureLock.Lock()
	if ctx.requestEpoch.Load()&1 != 0 {
		// 已关闭的生命周期不能再次按路径清理同名新实例的请求。
		s.futureLock.Unlock()
		return
	}
	ctx.requestEpoch.Add(1)
	s.futureLock.Unlock()
	// 关闭登记后再清理，后续请求无法越过最终快照。
	s.removeFuturesByAgentPath(ctx.ref.GetPath(), vivid.ErrorActorDeaded)
}

func (s *System) reopenContextRequests(ctx *Context) {
	s.futureLock.Lock()
	if ctx.requestEpoch.Load()&1 != 0 {
		ctx.requestEpoch.Add(1)
	}
	s.futureLock.Unlock()
}

func (s *System) closeFutureRequests() {
	s.futureLock.Lock()
	// 根 Actor 已结束，此后登记必须被拒绝；此前接纳的请求先复制，再在锁外完成。
	s.requestsClosed = true
	var pending []*future.Future[vivid.Message]
	for _, requests := range s.futureAgents {
		for path := range requests {
			if value, exists := s.actorContexts.Load(path); exists {
				if request, ok := value.(*future.Future[vivid.Message]); ok {
					pending = append(pending, request)
				}
			}
		}
	}
	s.futureLock.Unlock()
	for _, request := range pending {
		request.Close(vivid.ErrorActorSystemStopped)
	}
}

func (s *System) removeFuture(agentRef *AgentRef) {
	s.actorContexts.Delete(agentRef.ref.GetPath())

	s.futureLock.Lock()
	defer s.futureLock.Unlock()

	agentPath := agentRef.agent.GetPath()
	delete(s.futureAgents[agentPath], agentRef.ref.GetPath())
	if len(s.futureAgents[agentPath]) == 0 {
		delete(s.futureAgents, agentPath)
	}
}

func (s *System) removeFuturesByAgentPath(agentPath vivid.ActorPath, err error) {
	s.futureLock.Lock()
	// 完成回调会并发移除 Future，先在锁内复制路径，再在锁外执行关闭。
	refs := make([]vivid.ActorPath, 0, len(s.futureAgents[agentPath]))
	for ref := range s.futureAgents[agentPath] {
		refs = append(refs, ref)
	}
	s.futureLock.Unlock()

	for _, ref := range refs {
		if ctx, ok := s.actorContexts.Load(ref); ok {
			if f, ok := ctx.(*future.Future[vivid.Message]); ok {
				f.Close(err)
			}
		}
	}
}

// appendActorContext 用于添加指定路径的 ActorContext。
func (s *System) appendActorContext(ctx *Context) bool {
	s.actorInitLock.Lock()
	defer s.actorInitLock.Unlock()
	_, loaded := s.actorContexts.LoadOrStore(ctx.Ref().GetPath(), ctx)
	if !loaded {
		delete(s.initializingActors, ctx.Ref().GetPath())
	}
	return loaded
}

// reserveActorContext 在预启动前取得名称，避免失败实例影响同名 Actor 的资源。
func (s *System) reserveActorContext(ctx *Context) error {
	s.actorInitLock.Lock()
	defer s.actorInitLock.Unlock()
	path := ctx.Ref().GetPath()
	_, exists := s.actorContexts.Load(path)
	if path == "/" || exists || s.initializingActors[path] != nil {
		return vivid.ErrorActorAlreadyExists.WithMessage(path)
	}
	if s.initializingActors == nil {
		s.initializingActors = make(map[vivid.ActorPath]*Context)
	}
	s.initializingActors[path] = ctx
	return nil
}

// removeActorContext 只移除本实例的路由和关闭标记。
func (s *System) removeActorContext(ctx *Context) {
	s.actorContexts.CompareAndDelete(ctx.Ref().GetPath(), ctx)
	s.unmarkActorAsClosing(ctx.Ref())
}

// FindActor 根据引用字符串查找本节点上已存在的 Actor 并返回其引用。
// 仅支持本机地址：若字符串指向远程节点，或本机不存在该路径的 Actor，返回错误。
func (s *System) FindActor(actorRef string) (vivid.ActorRef, error) {
	ref, err := ParseRef(actorRef)
	if err != nil {
		return nil, err
	}
	if ref.GetAddress() != s.Ref().GetAddress() {
		return nil, vivid.ErrorNotFound
	}

	ctx, _ := s.actorContexts.Load(ref.GetPath())
	if v, ok := ctx.(*Context); ok {
		return v.ref, nil
	}

	return nil, vivid.ErrorNotFound
}

// ParseRef 将引用字符串解析为 ActorRef，不要求目标存在于本节点或远程。
func (s *System) ParseRef(actorRef string) (vivid.ActorRef, error) {
	return ParseRef(actorRef)
}

func (s *System) CreateRef(address string, path string) (vivid.ActorRef, error) {
	return NewRef(address, path)
}

func (s *System) Metrics() metrics.Metrics {
	if !s.options.EnableMetrics {
		return getDiscordMetrics()
	}
	return s.metrics
}

func (s *System) IsMetricsEnabled() bool {
	return s.options.EnableMetrics
}

// findMailbox 负责根据给定的 ActorRef 查找并返回对应的邮箱（Mailbox）。
func (s *System) findMailbox(ref *Ref) vivid.Mailbox {
	if ref == nil {
		return s.Mailbox()
	}

	isRemoteRef := ref.GetAddress() != s.Ref().GetAddress()

	// 尝试优先从 Ref 的 cache 字段中读取 Mailbox 指针，如果存在则直接返回，减少 map 查找的开销。
	if ptr := ref.cache.Load(); ptr != nil {
		cached := ptr.mailbox
		cacheRemote := isRemoteMailbox(cached)
		if cacheRemote == isRemoteRef {
			return cached
		}
	}

	// 检查是否为远程地址，使用远程邮箱
	if isRemoteRef {
		// 当系统上下文已关闭，直接返回系统根 Actor 的 Mailbox 作为兜底
		if s.options.Context.Err() != nil {
			return s.Mailbox()
		}
		if s.remotingRef == nil {
			s.Logger().Warn("remote disabled, remote actor ref not allowed", log.String("ref", ref.String()))
			return s.Mailbox()
		}
		return s.remotingMailbox
	}

	// 在 actorContexts 中查找指定路径（GetPath）对应的 Context，并尝试获取其邮箱（Mailbox）。
	if value, ok := s.actorContexts.Load(ref.GetPath()); ok {
		switch v := value.(type) {
		case *Context:
			m := v.Mailbox()
			ref.cacheMailbox(m)
			return m
		case *future.Future[vivid.Message]:
			return v
		}
	}
	m := s.Mailbox()
	ref.cacheMailbox(m)
	return m
}

func isRemoteMailbox(m vivid.Mailbox) bool {
	switch m.(type) {
	case *remoting.Mailbox:
		return true
	default:
		return false
	}
}

func (s *System) TellWithSender(sender vivid.ActorRef, recipient vivid.ActorRef, message vivid.Message) {
	s.tellWithSender(false, sender, recipient, message)
}

func (s *System) AskWithSender(sender vivid.ActorRef, recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message] {
	return s.askWithSender(false, sender, recipient, message, timeout...)
}

func (s *System) VirtualRef(kind string, name string) vivid.ActorRef {
	return virtual.NewIdentity(kind, name)
}
