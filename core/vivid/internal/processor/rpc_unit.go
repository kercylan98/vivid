package processor

import (
	"github.com/kercylan98/go-log/log"
	processor2 "github.com/kercylan98/vivid/core/vivid/processor"
	"github.com/kercylan98/vivid/src/queues"
	"math"
	"math/rand/v2"
	"sync/atomic"
	"time"
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

func NewRPCUnit(remoteId UnitIdentifier, conn processor2.RPCConn, config *RPCUnitConfiguration) Unit {
	if config.Logger == nil {
		config.Logger = log.GetDefault()
	}

	return &rpcUnit{
		config:   *config,
		remoteId: remoteId,
		conn:     conn,
		queue:    queues.NewRingBuffer(32),
	}
}

type rpcUnitMessage struct {
	address string
	path    string
	typ     string
	message []byte
}

// rpcUnit 是用于实现 RPC 功能的处理单元
type rpcUnit struct {
	config   RPCUnitConfiguration
	remoteId UnitIdentifier
	conn     processor2.RPCConn
	queue    *queues.RingBuffer
	num      int32
	status   uint32
}

func (r *rpcUnit) Logger() log.Logger {
	return r.config.Logger
}

func (r *rpcUnit) HandleUserMessage(sender UnitIdentifier, message any) {
	typ, data, err := r.config.Serializer.Serialize(message)
	if err != nil {
		r.Logger().Error("serialize", log.Err(err))
		return
	}

	r.queue.Push(&rpcUnitMessage{
		address: sender.GetAddress(),
		path:    sender.GetPath(),
		typ:     typ,
		message: data,
	})

	atomic.AddInt32(&r.num, 1)
	if atomic.CompareAndSwapUint32(&r.status, rpcUnitIdle, rpcUnitRunning) {
		go r.flush()
	}
}

func (r *rpcUnit) HandleSystemMessage(sender UnitIdentifier, message any) {
	r.HandleUserMessage(sender, message)
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

	var batch = processor2.NewRPCBatchMessage()

	for {
		if message, ok := r.queue.Pop().(*rpcUnitMessage); ok {
			atomic.AddInt32(&r.num, -1)
			batch.Add(message.address, message.path, message.typ, message.message)
			processed = true

			if batch.Len() >= r.config.BatchSize {
				r.publish(batch)
				batch = processor2.NewRPCBatchMessage()
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

func (r *rpcUnit) publish(batch processor2.RPCBatchMessage) {
	var failCount int

	for {
		buf, err := batch.Marshal()
		if err != nil {
			r.Logger().Error("serialize", log.Err(err))
			break
		}

		if err = r.conn.Send(buf); err == nil {
			r.Logger().Error("send", log.Err(err))
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
