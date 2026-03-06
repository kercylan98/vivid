package virtual

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/bridge"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ bridge.VirtualCoordinator = (*CoordinatorActor)(nil)
)

// CoordinatorInjecter 定义虚拟协调器的注入契约：在 ActorSystem 启动阶段
// 将协调器注册为 Actor，并阻塞直至其收到 OnLaunch、获得 ActorContext 后返回。
// 调用方据此保证协调器在就绪后才参与消息路由。
type CoordinatorInjecter interface {
	Inject(system vivid.ActorSystem) (*CoordinatorActor, error)
}

func NewCoordinatorActor(system bridge.VirtualActorSystem) CoordinatorInjecter {
	c := &CoordinatorActor{
		system:     system,
		activation: newActivation(system),
		launchWait: make(chan struct{}),
	}
	return c
}

type CoordinatorActor struct {
	system         bridge.VirtualActorSystem
	activation     *activation
	coordinatorCtx vivid.ActorContext
	launchWait     chan struct{}
}

func (c *CoordinatorActor) Inject(system vivid.ActorSystem) (*CoordinatorActor, error) {
	_, err := system.ActorOf(c, vivid.WithActorName("@virtual-coordinator"))
	if err != nil {
		return nil, err
	}
	<-c.launchWait
	return c, nil
}

func (c *CoordinatorActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		c.onLaunch(ctx)
	case *vivid.OnKilled:
		c.onKilled(ctx, msg)
	}
}

func (c *CoordinatorActor) onLaunch(ctx vivid.ActorContext) {
	// 该生命周期结束后 ActorSystem 的启动流程才会继续
	defer close(c.launchWait)

	c.coordinatorCtx = ctx
}

func (c *CoordinatorActor) onKilled(_ vivid.ActorContext, message *vivid.OnKilled) {
	c.activation.deactivate(message.Ref)
}

func (c *CoordinatorActor) Tell(sender, recipient vivid.ActorRef, message vivid.Message) {
	identity, ok := recipient.(*Identity)
	if !ok {
		return
	}
	ref, err := c.activation.activate(c.coordinatorCtx, identity)
	if err != nil {
		c.system.Logger().Error("failed to activate virtual actor",
			log.String("sender", sender.String()),
			log.String("kind", identity.kind),
			log.String("name", identity.name),
			log.Any("err", err),
		)
		return
	}
	c.system.TellWithSender(sender, ref, message)
}

func (c *CoordinatorActor) Ask(sender, recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message] {
	identity, ok := recipient.(*Identity)
	if !ok {
		return future.NewFutureFail[vivid.Message](vivid.ErrorVirtualRecipientException.WithMessage(recipient.String()))
	}
	ref, err := c.activation.activate(c.coordinatorCtx, identity)
	if err != nil {
		return future.NewFutureFail[vivid.Message](err)
	}
	return c.system.AskWithSender(sender, ref, message, timeout...)
}
