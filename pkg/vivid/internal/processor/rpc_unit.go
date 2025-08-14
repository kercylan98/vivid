package processor

import (
	"math"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/queues"
	"github.com/kercylan98/vivid/pkg/serializer"
	"github.com/kercylan98/vivid/pkg/vivid/processor"
)

const (
	rpcUnitIdle uint32 = iota
	rpcUnitRunning

	rpcUnitBaseDelay     = time.Millisecond * 100
	rpcUnitMaxDelay      = time.Second
	rpcUnitMultiplier    = 2.0
	rpcUnitRandomization = 0.5
)

var (
	_ Unit = (*rpcUnit)(nil)
)

func NewRPCUnit(remoteId UnitIdentifier, conn processor.RPCConn, config *RPCUnitConfiguration) (Unit, serializer.NameSerializer) {
	if config.Logger == nil {
		config.Logger = log.GetDefault()
	}
	s := config.SerializerProvider.Provide()
	return &rpcUnit{
		config:     *config,
		serializer: s,
		remoteId:   remoteId,
		conn:       conn,
		queue:      queues.NewRingBuffer(32),
	}, s
}

type rpcUnitMessage struct {
	sender  string
	target  string
	typ     string
	message []byte
	system  bool
}

// rpcUnit 是用于实现 RPC 功能的处理单元
type rpcUnit struct {
	config     RPCUnitConfiguration
	serializer serializer.NameSerializer
	remoteId   UnitIdentifier
	conn       processor.RPCConn
	queue      *queues.RingBuffer
	num        int32
	status     uint32
}

func (r *rpcUnit) Logger() log.Logger {
	return r.config.Logger
}

func (r *rpcUnit) HandleUserMessage(sender UnitIdentifier, message any) {
	r.handleMessage(sender, false, message)
}

func (r *rpcUnit) HandleSystemMessage(sender UnitIdentifier, message any) {
	r.handleMessage(sender, true, message)
}

func (r *rpcUnit) handleMessage(_ UnitIdentifier, system bool, message any) {
	sender, message := UnwrapMessage(message)
	typ, data, err := r.serializer.Serialize(message)
	if err != nil {
		r.Logger().Error("serialize", log.Err(err))
		return
	}

	unitMessage := &rpcUnitMessage{
		target:  r.remoteId.String(), // 指向本地创建该远程单元的 Actor
		typ:     typ,
		message: data,
		system:  system,
	}

	// 投递该消息的发送者 Actor
	if sender != nil {
		unitMessage.sender = sender.String()
	}

	r.queue.Push(unitMessage)

	atomic.AddInt32(&r.num, 1)
	if atomic.CompareAndSwapUint32(&r.status, rpcUnitIdle, rpcUnitRunning) {
		go r.flush()
	}
}

func (r *rpcUnit) flush() {
	// 持续处理直到没有消息为止
	for {
		processed := r.batchPack()

		// 如果这一轮没有处理任何消息，尝试退出
		if !processed {
			// 设置状态为 idle
			atomic.StoreUint32(&r.status, rpcUnitIdle)

			// 再次检查是否有新消息到达（避免在设置 idle 前有新消息入队的竞态条件）
			if atomic.LoadInt32(&r.num) > 0 {
				// 如果有新消息，尝试重新获取运行状态
				// 如果 CAS 失败，说明其他 goroutine 已经在处理了，可以安全退出
				if atomic.CompareAndSwapUint32(&r.status, rpcUnitIdle, rpcUnitRunning) {
					continue // 继续处理
				}
			}
			break
		}
	}
}

func (r *rpcUnit) batchPack() bool {
	processed := false

	var batch = processor.NewRPCBatchMessage()

	for {
		if message, ok := r.queue.Pop().(*rpcUnitMessage); ok {
			atomic.AddInt32(&r.num, -1)
			batch.Add(message.sender, message.target, message.typ, message.message, message.system)
			processed = true

			if batch.Len() >= r.config.BatchSize {
				r.publish(batch)
				batch = processor.NewRPCBatchMessage()
			}
			continue
		}
		break
	}

	if batch.Len() > 0 {
		r.publish(batch)
	}

	return processed
}

func (r *rpcUnit) publish(batch processor.RPCBatchMessage) {
	var failCount int

	for {
		buf, err := batch.Marshal()
		if err != nil {
			r.Logger().Error("serialize", log.Err(err))
			break
		}

		if err = r.conn.Send(buf); err == nil {
			break
		}
		// 退避重试，该重试没有必要存在上限，因为即便是达到次数上限跳出循环，也会被下一次消息发送重新激活而继续阻塞
		failCount++
		delay := float64(rpcUnitBaseDelay) * math.Pow(rpcUnitMultiplier, float64(failCount))
		jitter := (rand.Float64() - 0.5) * rpcUnitRandomization * float64(rpcUnitBaseDelay)
		sleepDuration := time.Duration(delay + jitter)
		if sleepDuration > rpcUnitMaxDelay {
			sleepDuration = rpcUnitMaxDelay
		}

		if failCount >= r.config.FailRetry {
			r.Logger().Error("send", log.Err(err))
			break
		}

		time.Sleep(sleepDuration)
	}
}
