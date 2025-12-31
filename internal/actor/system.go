package actor

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/mailbox"
	metricsActor "github.com/kercylan98/vivid/internal/metrics"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/sugar"
)

var (
	_ vivid.ActorSystem              = (*System)(nil)
	_ vivid.EnvelopHandler           = (*System)(nil)
	_ remoting.NetworkEnvelopHandler = (*System)(nil)
)

func NewSystem(options ...vivid.ActorSystemOption) *sugar.Result[*System] {
	return newSystem(nil, nil, options...)
}

func newSystem(testSystem *TestSystem, startBeforeHandler func(system *TestSystem), options ...vivid.ActorSystemOption) *sugar.Result[*System] {
	opts := vivid.NewActorSystemOptions(options...)

	system := &System{
		testSystem:        testSystem,
		options:           opts,
		actorContexts:     sync.Map{},
		guardClosedSignal: make(chan struct{}),
	}

	system.eventStream = newEventStream(system)

	var err error
	var logAttrs []any
	system.Context, err = NewContext(system, nil, guard.NewActor(system.guardClosedSignal))
	if err != nil {
		return sugar.Err[*System](err)
	}

	if startBeforeHandler != nil {
		startBeforeHandler(system.testSystem)
	}

	// 初始化指标收集 Actor
	if opts.EnableMetrics {
		var actorSystemMetrics metrics.Metrics
		if opts.Metrics != nil {
			actorSystemMetrics = opts.Metrics
		} else {
			actorSystemMetrics = metrics.NewDefaultMetrics()
		}
		system.metrics = actorSystemMetrics
		metricsActor := metricsActor.NewActor(opts.EnableMetricsUpdatedNotify)
		result := system.ActorOf(metricsActor, vivid.WithActorName("@metrics"))
		if result.IsErr() {
			return sugar.Err[*System](result.Err())
		}
		logAttrs = append(logAttrs, log.Bool("metrics_enabled", true))
	}

	// 初始化远程服务器 Actor
	if opts.RemotingBindAddress != "" && opts.RemotingAdvertiseAddress != "" {
		logAttrs = append(logAttrs, log.String("bind_address", opts.RemotingBindAddress))
		logAttrs = append(logAttrs, log.String("advertise_address", opts.RemotingAdvertiseAddress))

		var remotingServerActorOptions = remoting.ServerActorOptions{}
		if system.testSystem != nil {
			remotingServerActorOptions.ListenerCreatedHandler = func(listener net.Listener) {
				system.testSystem.onBindRemotingListener(listener)
			}
		}

		system.remotingServer = remoting.NewServerActor(opts.RemotingBindAddress, opts.RemotingAdvertiseAddress, opts.RemotingCodec, system, remotingServerActorOptions)
		result := system.ActorOf(system.remotingServer, vivid.WithActorName("@remoting"))
		if result.IsErr() {
			return sugar.Err[*System](result.Err())
		}
	}

	system.Logger().Debug("actor system initialized", logAttrs...)
	return sugar.With(system, nil)
}

type System struct {
	*Context                                    // ActorSystem 本身就表示了根 Actor
	testSystem        *TestSystem               // 测试系统
	options           *vivid.ActorSystemOptions // 系统选项
	actorContexts     sync.Map                  // 用于加速访问的 ActorContext 缓存（含有 Future）
	guardClosedSignal chan struct{}             // 用于通知系统关闭的信号
	remotingServer    *remoting.ServerActor     // 远程服务器
	eventStream       vivid.EventStream         // 事件流
	metrics           metrics.Metrics           // 指标收集器
}

// HandleRemotingEnvelop implements remoting.NetworkEnvelopHandler.
func (s *System) HandleRemotingEnvelop(system bool, agentAddr, agentPath, senderAddr, senderPath, receiverAddr, receiverPath string, messageInstance any) {
	var agent, sender, receiver *Ref
	if agentAddr != "" {
		// Addr 不为空就一定存在 Path
		agent = NewRef(agentAddr, agentPath)
	}
	sender = NewRef(senderAddr, senderPath)
	receiver = NewRef(receiverAddr, receiverPath)
	receiverMailbox := s.findMailbox(receiver)
	receiverMailbox.Enqueue(mailbox.NewEnvelop(system, sender, receiver, messageInstance).WithAgent(agent))
}

func (s *System) HandleEnvelop(envelop vivid.Envelop) {
	receiverMailbox := s.findMailbox(envelop.Sender().(*Ref))
	receiverMailbox.Enqueue(envelop)
}

func (s *System) Stop(timeout ...time.Duration) error {
	var stopTimeout = time.Minute
	if len(timeout) > 0 && timeout[0] > 0 {
		stopTimeout = timeout[0]
	}

	s.Logger().Debug("actor system stopping", log.Duration("timeout", stopTimeout))
	s.Context.Kill(s.Context.Ref(), true, "actor system stop")

	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	select {
	case <-s.guardClosedSignal:
		cancel()
	case <-ctx.Done():
		s.actorContexts.Range(func(key, value any) bool {
			return true
		})
		return fmt.Errorf("actor system stop timeout, %w", ctx.Err())
	}

	s.Logger().Debug("actor system stopped")
	return nil
}

func (s *System) appendFuture(agentRef *AgentRef, future *future.Future[vivid.Message]) {
	s.actorContexts.Store(agentRef.ref.GetPath(), future)
}

func (s *System) removeFuture(agentRef *AgentRef) {
	s.actorContexts.Delete(agentRef.ref.GetPath())
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

func (s *System) Metrics() metrics.Metrics {
	if s.metrics == nil {
		s.Logger().Warn("metrics not enabled, returning temporary metrics collector, should use vivid.WithActorSystemEnableMetrics to enable metrics")
		return metrics.NewDefaultMetrics()
	}
	return s.metrics
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
