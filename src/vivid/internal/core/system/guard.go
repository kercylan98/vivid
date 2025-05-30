package system

import (
	"context"
	"time"

	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

var _ actor.Actor = (*Guard)(nil)

func GuardProvider(cancel context.CancelFunc) actor.Provider {
	return actor.ProviderFN(func() actor.Actor {
		return &Guard{cancel: cancel}
	})
}

type Guard struct {
	cancel context.CancelFunc
}

// recordSystemMessageReceived 优雅地记录系统消息接收
func (g *Guard) recordSystemMessageReceived(ctx actor.Context, messageType string, startTime time.Time) {
	if config := ctx.MetadataContext().Config(); config.Monitoring != nil {
		latency := time.Since(startTime).Nanoseconds()
		config.Monitoring.RecordSystemMessageReceived(ctx.MetadataContext().Ref(), messageType, latency)
	}
}

func (g *Guard) OnReceive(ctx actor.Context) {
	startTime := time.Now() // 统一记录消息处理开始时间

	switch msg := ctx.MessageContext().Message().(type) {
	case *actor.OnKill:
		g.recordSystemMessageReceived(ctx, "OnKill", startTime)

		// 收到Kill消息，记录并处理
		ctx.MetadataContext().Config().LoggerProvider.Provide().Debug(
			"Guard Actor received OnKill message",
			"reason", msg.Reason,
			"poison", msg.Poison,
		)

		if msg.Poison {
			// 优雅关闭：停止所有子Actor并等待消息处理完毕
			g.gracefulShutdown(ctx)
		}
		// 对于非poison的OnKill，让生命周期自动处理

	case *actor.OnKilled:
		g.recordSystemMessageReceived(ctx, "OnKilled", startTime)

		// Actor被杀死后的清理工作
		ctx.MetadataContext().Config().LoggerProvider.Provide().Debug(
			"Guard Actor terminated, canceling system context",
		)
		g.cancel()

	case *actor.OnLaunch:
		g.recordSystemMessageReceived(ctx, "OnLaunch", startTime)

		// 守护Actor启动
		ctx.MetadataContext().Config().LoggerProvider.Provide().Debug(
			"Guard Actor launched",
		)
	}
}

// gracefulShutdown 执行优雅关闭：停止所有子Actor并等待消息处理完毕
func (g *Guard) gracefulShutdown(ctx actor.Context) {
	logger := ctx.MetadataContext().Config().LoggerProvider.Provide()
	logger.Debug("Starting graceful shutdown of all child actors")

	// 0. 立即停止监控计数，在发送任何关闭消息之前！
	system := ctx.MetadataContext().System()
	if systemImpl, ok := system.(*System); ok {
		if globalMonitoring := systemImpl.GetGlobalMonitoring(); globalMonitoring != nil {
			// 尝试停止监控计数
			if stoppableMetrics, ok := globalMonitoring.(interface{ StopRecording() }); ok {
				stoppableMetrics.StopRecording()
				logger.Debug("🔴 Stopped monitoring recording BEFORE sending any shutdown messages")
			}
		}
	}

	// 1. 停止所有子Actor
	children := ctx.RelationContext().Children()
	if len(children) > 0 {
		logger.Debug("Sending OnKill to all child actors", "count", len(children))
		for _, child := range children {
			// 发送poison kill给每个子Actor - 使用 UserMessage 确保优雅关闭
			ctx.TransportContext().Tell(child, actx.UserMessage, &actor.OnKill{
				Reason:   "system graceful shutdown",
				Operator: ctx.MetadataContext().Ref(),
				Poison:   true,
			})
		}
	}

	logger.Debug("Graceful shutdown completed, terminating guard")

	// 2. 最后终止自己（这个OnKill不会被计数，因为已经StopRecording）
	ctx.TransportContext().Tell(ctx.MetadataContext().Ref(), actx.SystemMessage, &actor.OnKill{
		Reason:   "guard shutdown complete",
		Operator: ctx.MetadataContext().Ref(),
		Poison:   false,
	})
}
