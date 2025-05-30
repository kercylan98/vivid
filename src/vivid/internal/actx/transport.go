package actx

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	futureImpl "github.com/kercylan98/vivid/src/vivid/internal/future"
	"github.com/kercylan98/wasteland/src/wasteland"
)

var _ actor.TransportContext = (*Transport)(nil)

func NewTransport(ctx actor.Context) *Transport {
	return &Transport{
		ctx: ctx,
	}
}

type Transport struct {
	ctx actor.Context
}

// getMessageTypeName 获取消息类型名称，用于监控记录
func (t *Transport) getMessageTypeName(message interface{}) string {
	if message == nil {
		return "nil"
	}

	// 使用反射获取类型名
	msgType := reflect.TypeOf(message)
	if msgType.Kind() == reflect.Ptr {
		msgType = msgType.Elem()
	}

	// 获取简短的类型名（不包含包名）
	typeName := msgType.Name()
	if typeName == "" {
		// 如果没有名称，使用完整类型字符串
		typeName = msgType.String()
	}

	return typeName
}

// recordMessageSent 记录消息发送到监控系统
func (t *Transport) recordMessageSent(target actor.Ref, message core.Message, priority wasteland.MessagePriority) {
	config := t.ctx.MetadataContext().Config()
	monitoring := config.Monitoring

	// 如果当前上下文没有监控配置，尝试从系统获取全局监控
	if monitoring == nil {
		if globalMonitoring := t.ctx.MetadataContext().System().GetGlobalMonitoring(); globalMonitoring != nil {
			// 尝试类型断言为监控接口
			if m, ok := globalMonitoring.(actor.Metrics); ok {
				monitoring = m
			}
		}
	}

	if monitoring != nil {
		from := t.ctx.MetadataContext().Ref()
		messageType := t.getMessageTypeName(message)

		// 根据优先级区分用户消息和系统消息
		if priority == UserMessage {
			monitoring.RecordUserMessageSent(from, target, messageType)
		} else {
			monitoring.RecordSystemMessageSent(from, target, messageType)
		}
	}
}

// HandlePing 处理 OnPing 消息
func (t *Transport) HandlePing(msg *actor.OnPing) {
	// 自动响应 ping 消息，返回 pong 消息
	pong := &actor.OnPong{
		OriginalTimestamp: msg.Timestamp,
		Timestamp:         time.Now().UnixNano(),
	}
	t.Reply(core.SystemMessage, pong)
}

// HandlePong 处理 OnPong 消息
func (t *Transport) HandlePong(msg *actor.OnPong, sender actor.Ref) {
	// 计算往返时间（毫秒）
	rtt := (time.Now().UnixNano() - msg.OriginalTimestamp) / int64(time.Millisecond)
	// 使用往返时间完成 future
	if sender != nil {
		t.Tell(sender, core.SystemMessage, rtt)
	}
}

func (t *Transport) Tell(target actor.Ref, priority wasteland.MessagePriority, message core.Message) {
	// 记录消息发送到监控系统
	t.recordMessageSent(target, message, priority)

	t.ctx.MetadataContext().System().Find(target).HandleMessage(nil, priority, message)
}

func (t *Transport) Probe(target actor.Ref, priority wasteland.MessagePriority, message core.Message) {
	// 记录消息发送到监控系统
	t.recordMessageSent(target, message, priority)

	t.ctx.MetadataContext().System().Find(target).HandleMessage(t.ctx.MetadataContext().Ref(), priority, message)
}

func (t *Transport) Ask(target actor.Ref, priority wasteland.MessagePriority, message core.Message, timeout ...time.Duration) future.Future {
	// 记录消息发送到监控系统
	t.recordMessageSent(target, message, priority)

	d := time.Second
	if len(timeout) > 0 {
		d = timeout[0]
	}

	meta := t.ctx.MetadataContext()
	futureRef := meta.Ref().GenerateSub(strconv.FormatInt(t.ctx.RelationContext().NextGuid(), 10))
	f := futureImpl.New(meta.System().Registry(), futureRef, d)
	meta.System().Find(target).HandleMessage(f.GetID(), priority, message)
	return f
}

func (t *Transport) Reply(priority wasteland.MessagePriority, message core.Message) {
	sender := t.ctx.MessageContext().Sender()

	// 记录回复消息发送到监控系统
	if sender != nil {
		t.recordMessageSent(sender, message, priority)
	}

	t.ctx.TransportContext().Tell(sender, priority, message)

	if sender == nil {
		t.ctx.LoggerProvider().Provide().Warn("reply",
			log.String("ref", t.ctx.MetadataContext().Ref().String()),
			log.Err(fmt.Errorf("tell message can not reply, but reply message %T", message)))
	}
}

// Ping 向目标 Actor 发送 ping 消息并等待 pong 响应。
// 它直接返回 Pong 结构体和可能的错误。
// 如果目标 Actor 不可达或者超时，将返回错误。
// 如果未指定超时时间，默认为 1 秒。
func (t *Transport) Ping(target actor.Ref, timeout ...time.Duration) (*actor.OnPong, error) {
	// 检查目标是否为空
	if target == nil {
		return nil, errors.New("ping target is nil")
	}

	d := time.Second
	if len(timeout) > 0 {
		d = timeout[0]
	}

	// 创建带有当前时间戳的 ping 消息
	ping := &actor.OnPing{
		Timestamp: time.Now().UnixNano(),
	}

	// 记录ping消息发送到监控系统
	t.recordMessageSent(target, ping, core.SystemMessage)

	// 发送 ping 消息并等待 pong 响应
	meta := t.ctx.MetadataContext()
	futureRef := meta.Ref().GenerateSub(strconv.FormatInt(t.ctx.RelationContext().NextGuid(), 10))
	f := futureImpl.New(meta.System().Registry(), futureRef, d)
	meta.System().Find(target).HandleMessage(f.GetID(), core.SystemMessage, ping)

	// 等待 Future 完成
	result, err := f.Result()
	if err != nil {
		return nil, err
	}

	return result.(*actor.OnPong), nil
}
