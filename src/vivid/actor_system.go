package vivid

import (
	"fmt"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/system"
	"strings"
	"time"
)

// ActorSystem 是 Actor 的容器和管理者，负责 Actor 的创建、监督和消息传递，
// 它提供了与 Actor 交互的主要接口，包括创建 Actor、发送消息、管理系统生命周期等功能，
// 一个应用程序通常只有一个 ActorSystem 实例。
type ActorSystem interface {
	context

	// Start 启动 ActorSystem，
	// 返回可能发生的错误，如果启动成功则返回 nil，
	// 在调用此方法后，系统将开始处理消息和管理 Actor。
	Start() error

	// StartP 启动 ActorSystem，并在启动失败时引发 panic，
	// 返回 ActorSystem 自身，便于链式调用，
	// 这是一个便捷方法，适用于确信启动不会失败或希望在启动失败时立即终止程序的场景。
	StartP() ActorSystem

	// Stop 停止 ActorSystem，
	// 返回可能发生的错误，如果停止成功则返回 nil，
	// 在调用此方法后，系统将停止处理新消息，并尝试优雅地关闭所有 Actor。
	Stop() error

	// StopP 停止 ActorSystem，并在停止失败时引发 panic，
	// 这是一个便捷方法，适用于确信停止不会失败或希望在停止失败时立即终止程序的场景。
	StopP()

	// Ping 向特定的 Actor 发送 ping 消息并等待 pong 响应，
	//
	// 如果目标 Actor 不可达或者超时，将返回错误。
	//
	// 这个方法可以用来检测网络可用性并获取详细的响应信息。
	Ping(target ActorRef, timeout ...time.Duration) (*Pong, error)

	// RegisterPlugin 注册一个插件到 ActorSystem，
	// 如果插件已经注册，则返回错误。
	// 插件将在系统启动时初始化，在系统停止时关闭。
	RegisterPlugin(plugin Plugin) error

	// GetPlugin 获取指定 ID 的插件，
	// 如果插件不存在，则返回 nil 和错误。
	GetPlugin(id string) (Plugin, error)

	// GetPlugins 获取所有已注册的插件。
	GetPlugins() []Plugin
}

// NewActorSystem 创建一个新的 ActorSystem 实例，
// 可以通过提供 ActorSystemConfigurator 来配置系统的各种参数。
func NewActorSystem(configurator ...ActorSystemConfigurator) ActorSystem {
	config := newActorSystemConfig()
	for _, c := range configurator {
		c.Configure(config)
	}
	return &actorSystem{
		system:  system.New(*config.config),
		plugins: newPluginRegistry(),
	}
}

// actorSystem 是 ActorSystem 接口的实现，
// 它封装了底层的 actor.System 实例，并提供了符合 ActorSystem 接口的方法。
type actorSystem struct {
	system  actor.System    // 底层系统实例
	plugins *PluginRegistry // 插件注册表
}

func (a *actorSystem) Logger() log.Logger {
	return a.system.Context().MetadataContext().Config().LoggerProvider.Provide()
}

func (a *actorSystem) Kill(ref ActorRef, reason ...string) {
	ctx := a.system.Context()
	ctx.TransportContext().Tell(ref.(actor.Ref), actx.SystemMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: ctx.MetadataContext().Ref(),
	})
}

func (a *actorSystem) PoisonKill(ref ActorRef, reason ...string) {
	ctx := a.system.Context()
	ctx.TransportContext().Tell(ref.(actor.Ref), actx.UserMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: ctx.MetadataContext().Ref(),
		Poison:   true,
	})
}

func (a *actorSystem) Tell(target ActorRef, message Message) {
	a.system.Context().TransportContext().Tell(target.(actor.Ref), actx.UserMessage, message)
}

func (a *actorSystem) Probe(target ActorRef, message Message) {
	a.system.Context().TransportContext().Probe(target.(actor.Ref), actx.UserMessage, message)
}

func (a *actorSystem) Ask(target ActorRef, message Message, timeout ...time.Duration) Future {
	return a.system.Context().TransportContext().Ask(target.(actor.Ref), actx.UserMessage, message, timeout...)
}

func (a *actorSystem) Ping(target ActorRef, timeout ...time.Duration) (*Pong, error) {
	return a.system.Context().TransportContext().Ping(target.(actor.Ref), timeout...)
}

func (a *actorSystem) Start() error {
	// 启动 Actor 系统
	if err := a.system.Run(); err != nil {
		return err
	}

	// 初始化所有插件
	if err := a.plugins.Initialize(a); err != nil {
		return fmt.Errorf("failed to initialize plugins: %w", err)
	}

	return nil
}

func (a *actorSystem) StartP() ActorSystem {
	if err := a.Start(); err != nil {
		panic(err)
	}
	return a
}

func (a *actorSystem) Stop() error {
	// 先停止 Actor 系统
	if err := a.system.Stop(); err != nil {
		return err
	}

	// 关闭所有插件
	if err := a.plugins.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown plugins: %w", err)
	}

	return nil
}

func (a *actorSystem) StopP() {
	if err := a.Stop(); err != nil {
		panic(err)
	}
}

func (a *actorSystem) ActorOf(provider ActorProviderFN, configurator ...ActorConfiguratorFN) ActorRef {
	return a.ActorOfP(provider, configurator...)
}

func (a *actorSystem) ActorOfP(provider ActorProvider, configurator ...ActorConfiguratorFN) ActorRef {
	var cs = make([]ActorConfigurator, len(configurator))
	for i, cfg := range configurator {
		cs[i] = cfg
	}
	return a.ActorOfPC(provider, cs...)
}

func (a *actorSystem) ActorOfC(provider ActorProviderFN, configurator ...ActorConfigurator) ActorRef {
	return a.ActorOfPC(provider, configurator...)
}

func (a *actorSystem) ActorOfPC(provider ActorProvider, configurator ...ActorConfigurator) ActorRef {
	if len(configurator) > 0 {
		return newActorFacade(a.system, a.system.Context(), provider, configurator...)
	}
	return newActorFacade(a.system, a.system.Context(), provider)
}

// RegisterPlugin 注册一个插件到 ActorSystem
func (a *actorSystem) RegisterPlugin(plugin Plugin) error {
	return a.plugins.Register(plugin)
}

// GetPlugin 获取指定 ID 的插件
func (a *actorSystem) GetPlugin(id string) (Plugin, error) {
	return a.plugins.Get(id)
}

// GetPlugins 获取所有已注册的插件
func (a *actorSystem) GetPlugins() []Plugin {
	return a.plugins.GetAll()
}
