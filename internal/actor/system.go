package actor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/chain"
	"github.com/kercylan98/vivid/internal/cluster"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
)

const (
	ready int32 = iota
	start
	stop
)

var (
	_ vivid.ActorSystem              = (*System)(nil)
	_ vivid.EnvelopHandler           = (*System)(nil)
	_ remoting.NetworkEnvelopHandler = (*System)(nil)
	_ vivid.SystemStateProvider      = (*System)(nil)
	_ vivid.MetricsProvider          = (*System)(nil)
)

func NewSystem(options ...vivid.ActorSystemOption) *System {
	opts := vivid.NewActorSystemOptions(options...)

	system := &System{
		options:           opts,
		actorContexts:     sync.Map{},
		futureAgents:      make(map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef),
		guardClosedSignal: make(chan struct{}),
		scheduler:         scheduler.NewScheduler(opts.Context),
	}

	system.options.Context, system.cancel = context.WithCancel(system.options.Context)

	system.eventStream = newEventStream(system)
	return system
}

type System struct {
	*Context                                                            // ActorSystem 本身就表示了根 Actor
	actorOfLock       sync.Mutex                                        // ActorOf 方法的锁，保证 ActorOf 方法的并发安全
	options           *vivid.ActorSystemOptions                         // 系统选项
	actorContexts     sync.Map                                          // 用于加速访问的 ActorContext 缓存（含有 Future）
	futureAgents      map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef // 记录本地 Actor 其正在被代理的 Future
	futureLock        sync.Mutex                                        // Future 的锁，保证 Future 的并发安全
	guardClosedSignal chan struct{}                                     // 用于通知系统关闭的信号
	remotingServer    *remoting.ServerActor                             // 远程服务器
	eventStream       vivid.EventStream                                 // 事件流
	metrics           metrics.Metrics                                   // 指标收集器
	scheduler         *scheduler.Scheduler                              // 调度器
	status            int32                                             // 系统状态
	statusLock        sync.Mutex                                        // 系统状态锁
	clusterContext    *cluster.Context                                  // 集群上下文
	cancel            context.CancelFunc                                // 上下文停止函数
	startTime         time.Time                                         // 启动时间，Start() 时记录
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

func (s *System) IsClusterEnabled() bool {
	return s.clusterContext != nil
}

func (s *System) Cluster() vivid.ClusterContext {
	return s.clusterContext
}

func (s *System) HandleRemotingEnvelop(system bool, senderAddr, senderPath, receiverAddr, receiverPath string, messageInstance any) error {
	var sender, receiver *Ref
	var err error
	sender, err = NewRef(senderAddr, senderPath)
	if err != nil {
		s.Logger().Warn("invalid sender ref", log.String("address", senderAddr), log.String("path", senderPath), log.Any("err", err))
		return fmt.Errorf("%w: invalid sender ref, %s/%s", err, senderAddr, senderPath)
	}
	receiver, err = NewRef(receiverAddr, receiverPath)
	if err != nil {
		s.Logger().Warn("invalid receiver ref", log.String("address", receiverAddr), log.String("path", receiverPath), log.Any("err", err))
		return fmt.Errorf("%w: invalid receiver ref, %s/%s", err, receiverAddr, receiverPath)
	}
	receiverMailbox := s.findMailbox(receiver)
	envelop := mailbox.NewEnvelop(system, sender, receiver, messageInstance)
	receiverMailbox.Enqueue(envelop)
	return nil
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

func (s *System) ActorOf(actor vivid.Actor, options ...vivid.ActorOption) (vivid.ActorRef, error) {
	s.actorOfLock.Lock()
	defer s.actorOfLock.Unlock()

	return s.Context.ActorOf(actor, options...)
}

func (s *System) Start() error {
	var stateError = func(s *System) error {
		s.statusLock.Lock()
		defer s.statusLock.Unlock()
		switch s.status {
		case start:
			s.Logger().Warn("actor system already started")
			return vivid.ErrorActorSystemAlreadyStarted
		case stop:
			s.Logger().Warn("actor system already stopped")
			return vivid.ErrorActorSystemAlreadyStopped
		default:
			s.status = start
			return nil
		}
	}(s)
	if stateError != nil {
		return stateError
	}

	s.startTime = time.Now()
	s.Logger().Debug("actor system starting")

	startErr := chain.New(chain.WithContext(s.options.Context)).
		Append(systemChains.spawnGuardActor(s)).
		Append(systemChains.initializeMetrics(s)).
		Append(systemChains.initializeRemoting(s)).
		Append(systemChains.initializeCluster(s)).
		Run()

	if startErr != nil {
		s.Logger().Error("actor system start failed", log.Any("err", startErr))
		return vivid.ErrorActorSystemStartFailed.With(s.Stop(s.options.StopTimeout))
	}

	s.Logger().Debug("actor system started")

	// 守护系统上下文
	go func() {
		<-s.options.Context.Done()
		s.statusLock.Lock()
		defer s.statusLock.Unlock()
		_ = s.stop(false) // 无意义错误
	}()
	return nil
}
func (s *System) Stop(timeout ...time.Duration) error {
	return s.stop(true, timeout...)
}

func (s *System) stop(checkLog bool, timeout ...time.Duration) error {
	// 将锁范围限定在函数内部校验状态，避免每次 return 都重复编写锁释放代码
	var stateError = func(s *System) error {
		s.statusLock.Lock()
		defer s.statusLock.Unlock()
		switch s.status {
		case ready:
			if checkLog {
				s.Logger().Warn("actor system not started")
			}
			return vivid.ErrorActorSystemNotStarted
		case stop:
			if checkLog {
				s.Logger().Warn("actor system already stopped")
			}
			return vivid.ErrorActorSystemAlreadyStopped
		default:
			s.status = stop
			return nil
		}
	}(s)
	if stateError != nil {
		return stateError
	}

	// 优先离开集群（未启用集群时 clusterContext 为 nil）
	if s.clusterContext != nil {
		s.clusterContext.Leave()
	}

	var stopTimeout = sugar.Max(sugar.FirstOrDefault(timeout, s.options.StopTimeout), 0)
	s.Logger().Debug("actor system stopping", log.Duration("timeout", stopTimeout))

	if s.Context != nil {
		s.Context.Kill(s.Context.Ref(), true, "actor system stop")
		s.cancel()
		select {
		case <-s.guardClosedSignal:
			break
		case <-time.After(stopTimeout):
			s.Logger().Error("actor system stop failed", log.Duration("timeout", stopTimeout))
			return vivid.ErrorActorSystemStopFailed.With(context.DeadlineExceeded)
		}
	}

	// 清理调度器
	s.scheduler.Stop()

	s.Logger().Debug("actor system stopped")
	return nil
}

func (s *System) appendFuture(agentRef *AgentRef, future *future.Future[vivid.Message]) {
	s.actorContexts.Store(agentRef.ref.GetPath(), future)

	s.futureLock.Lock()
	defer s.futureLock.Unlock()

	agentPath := agentRef.agent.GetPath()
	futureMap, exists := s.futureAgents[agentPath]
	if !exists {
		futureMap = make(map[vivid.ActorPath]*AgentRef)
		s.futureAgents[agentPath] = futureMap
	}
	futureMap[agentRef.ref.GetPath()] = agentRef
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
	refs := s.futureAgents[agentPath]
	s.futureLock.Unlock()

	for ref := range refs {
		if ctx, ok := s.actorContexts.Load(ref); ok {
			if f, ok := ctx.(*future.Future[vivid.Message]); ok {
				f.Close(err)
			}
		}
	}
}

// appendActorContext 用于添加指定路径的 ActorContext。
func (s *System) appendActorContext(ctx *Context) bool {
	_, loaded := s.actorContexts.LoadOrStore(ctx.Ref().GetPath(), ctx)
	return loaded
}

// removeActorContext 用于移除指定路径的 ActorContext。
func (s *System) removeActorContext(ctx *Context) {
	s.actorContexts.Delete(ctx.Ref().GetPath())
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
		s.Logger().Warn("metrics not enabled, returning temporary metrics collector, should use vivid.WithActorSystemEnableMetrics to enable metrics")
		return metrics.NewDefaultMetrics()
	}
	return s.metrics
}

func (s *System) MetricsEnabled() bool {
	return s.options.EnableMetrics
}

// findMailbox 负责根据给定的 ActorRef 查找并返回对应的邮箱（Mailbox）。
func (s *System) findMailbox(ref *Ref) vivid.Mailbox {
	if ref == nil {
		// 若传入的引用为 nil，直接返回系统根 Actor 的邮箱作为兜底。
		return s.Mailbox()
	}

	// 尝试优先从 Ref 的 cache 字段中读取 Mailbox 指针，如果存在则直接返回，减少 map 查找的开销。
	if ptr := ref.cache.Load(); ptr != nil {
		return *ptr
	}

	// 检查是否为远程地址，使用远程邮箱
	if ref.GetAddress() != s.Ref().GetAddress() {
		// 当系统上下文已关闭，直接返回系统根 Actor 的 Mailbox 作为兜底
		if s.options.Context.Err() != nil {
			return s.Mailbox()
		}
		if s.remotingServer == nil {
			s.Logger().Warn("remote disabled, remote actor ref not allowed", log.String("ref", ref.String()))
			return s.Mailbox()
		}
		return s.remotingServer.GetRemotingMailboxCentral().GetOrCreate(ref.address, s)
	}

	// 在 actorContexts 中查找指定路径（GetPath）对应的 Context，并尝试获取其邮箱（Mailbox）。
	if value, ok := s.actorContexts.Load(ref.GetPath()); ok {
		switch v := value.(type) {
		case *Context:
			// 利用 CompareAndSwap 保证仅存储一次 Mailbox 指针到 cache，提升后续命中率，防止多线程下的闭包问题。
			m := v.Mailbox()
			ref.cache.CompareAndSwap(nil, &m)
			return m
		case *future.Future[vivid.Message]:
			return v
		}
	}
	// 若上述皆未命中，返回系统根 Actor 的 Mailbox 作为默认兜底方案，保证 Mailbox 一定可用。
	return s.Mailbox()
}
