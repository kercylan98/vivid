package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/internal/protobuf/protobuf"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
)

const (
	remoteStreamProcessStateIdle uint32 = iota
	remoteStreamProcessStateActive
	remoteStreamBatchLimit = 256
)

var _ Process = (*remoteStreamProcess)(nil)

func newRemoteStreamProcess(manager *remoteStreamManager, id ID) Process {
	rsp := &remoteStreamProcess{
		manager: manager,
		id:      id,
	}

	return rsp
}

type remoteStreamProcess struct {
	manager        *remoteStreamManager // 远程流管理器
	id             ID                   // 指向的远程流的 ID
	stream         remoteStream         // 远程流
	batch          []Envelope           // 批量消息
	rw             sync.RWMutex         // 读写锁
	state          atomic.Uint32        // 状态
	recoveryWaiter sync.WaitGroup       // 恢复等待组
}

func (r *remoteStreamProcess) GetID() ID {
	return r.id
}

func (r *remoteStreamProcess) Send(envelope Envelope) {
	r.recoveryWaiter.Wait()

	r.rw.Lock()
	r.batch = append(r.batch, envelope)
	r.rw.Unlock()

	r.activation()
}

func (r *remoteStreamProcess) Terminated() bool {
	return false
}

func (r *remoteStreamProcess) OnTerminate(operator ID) {}

func (r *remoteStreamProcess) activation() {
	if r.state.CompareAndSwap(remoteStreamProcessStateIdle, remoteStreamProcessStateActive) {
		go func() {
			for {
				stop := r.send()
				r.state.Store(remoteStreamProcessStateIdle)
				if stop {
					break
				}
				r.rw.RLock()
				empty := len(r.batch) == 0
				r.rw.RUnlock()
				if empty {
					break
				} else if !r.state.CompareAndSwap(remoteStreamProcessStateIdle, remoteStreamProcessStateActive) {
					break
				}
			}
		}()
	}
}

func (r *remoteStreamProcess) send() (stop bool) {
	const baseDelay = time.Millisecond * 100
	const maxDelay = time.Second
	const multiplier = 2.0
	const randomization = 0.5

	for {
		r.rw.Lock()
		n := len(r.batch)
		var batch []Envelope
		if n < remoteStreamBatchLimit {
			batch = r.batch
			r.batch = nil
		} else {
			batch = r.batch[:remoteStreamBatchLimit]
			r.batch = r.batch[remoteStreamBatchLimit:]
		}
		r.rw.Unlock()
		if len(batch) == 0 {
			break
		}

		// 尝试发送消息
		var stream = r.stream
		var err error
		var once sync.Once
		var failCount int
		var m *protobuf.Message

		for {
			// 尚未持有远程流，尝试获取
			if stream == nil {
				stream, err = r.manager.loadOrInitClientRemoteStream(r.id.GetHost())
				r.stream = stream
				if err != nil {
					r.manager.processManager.logger().Error("remote", log.String("event", "send"), log.String("addr", r.id.GetHost()), log.String("info", "get remote stream error"), log.Err(err))
					break
				}
			}

			// 如果获取远程流失败或者发送消息失败，进入下一次重试
			if err == nil {
				if m == nil {
					var batchBytes = make([][]byte, 0, len(batch))
					for _, envelope := range batch {
						data, err := stream.getCodec().Encode(envelope)
						if err != nil {
							panic(err)
						}
						batchBytes = append(batchBytes, data)
					}
					m = &protobuf.Message{MessageType: &protobuf.Message_Batch_{Batch: &protobuf.Message_Batch{Messages: batchBytes}}}
				}

				if err = stream.Send(m); err != nil {
					r.stream = nil
					r.stream.close()
				} else {
					// 发送成功，退出循环
					break
				}
			}

			// 发送失败，等待循环重试，阻塞后续消息，避免消息大量堆积
			once.Do(func() {
				r.recoveryWaiter.Add(1)
			})

			// 退避重试，该重试没有必要存在上限，因为即便是达到次数上限跳出循环，也会被下一次消息发送重新激活而继续阻塞
			failCount++
			delay := float64(baseDelay) * math.Pow(multiplier, float64(failCount))
			jitter := (rand.Float64() - 0.5) * randomization * float64(baseDelay)
			sleepDuration := time.Duration(delay + jitter)
			if sleepDuration > maxDelay {
				sleepDuration = maxDelay
			}

			r.manager.processManager.logger().Error("remote", log.String("event", "send"), log.String("addr", r.id.GetHost()), log.Int("times", failCount), log.String("info", "send message error"), log.Err(err))

			time.Sleep(sleepDuration)
		}

		// 解除等待
		if failCount > 0 {
			r.recoveryWaiter.Done()
		}
	}
	return
}
