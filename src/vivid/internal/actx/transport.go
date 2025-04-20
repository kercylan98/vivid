package actx

import (
    "errors"
    "fmt"
    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/vivid/src/vivid/internal/core"
    "github.com/kercylan98/vivid/src/vivid/internal/core/actor"
    "github.com/kercylan98/vivid/src/vivid/internal/core/future"
    futureImpl "github.com/kercylan98/vivid/src/vivid/internal/future"
    "github.com/kercylan98/wasteland/src/wasteland"
    "strconv"
    "time"
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
    t.ctx.MetadataContext().System().Find(target).HandleMessage(nil, priority, message)
}

func (t *Transport) Probe(target actor.Ref, priority wasteland.MessagePriority, message core.Message) {
    t.ctx.MetadataContext().System().Find(target).HandleMessage(t.ctx.MetadataContext().Ref(), priority, message)
}

func (t *Transport) Ask(target actor.Ref, priority wasteland.MessagePriority, message core.Message, timeout ...time.Duration) future.Future {
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
