package mailbox

import (
    "sync"
    "sync/atomic"
    "testing"
    "time"

    "github.com/kercylan98/vivid/src/vivid/internal/core"
    "github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
)

var _ mailbox.Handler = (*TestMailboxHandler)(nil)

type TestMailboxHandler struct{}

func (t *TestMailboxHandler) HandleSystemMessage(message core.Message) {
    message.(func())()
}

func (t *TestMailboxHandler) HandleUserMessage(message core.Message) {
    message.(func())()
}

func TestMailbox(t *testing.T) {
    type TestArgs struct {
        ProducerNum int
        MessageNum  int
        Timeout     time.Duration
    }
    var testArgs = []TestArgs{
        {100, 10000, time.Second * 5},
        {1000, 100000, time.Second * 10},
        {10000, 1000000, time.Second * 20},
    }

    for _, arg := range testArgs {
        deadLoopTestExecutor(t, newMailbox(), arg.ProducerNum, arg.MessageNum, arg.Timeout)
    }
}

func deadLoopTestExecutor(t *testing.T, mb *mailboxImpl, producerNum int, totalMessageNum int, timeout time.Duration) {
    var (
        globalWait        sync.WaitGroup
        done              atomic.Bool
        m                 = make(map[int64]struct{})
        mu                sync.Mutex   // 保护map m的互斥锁
        n                 atomic.Int64 // 使用atomic类型
        wg                sync.WaitGroup
        stopSignal        = make(chan struct{})
        exitSignal        = make(chan struct{})
        getNotRepeatedNum = func() int {
            mu.Lock()
            defer mu.Unlock()
            return len(m)
        }
        addToMap = func(v int64) bool {
            mu.Lock()
            defer mu.Unlock()
            _, exist := m[v]
            if exist {
                return false // 重复
            }
            m[v] = struct{}{}
            return true
        }
    )

    mb.Initialize(mailbox.DispatcherFN(func(f func()) {
        go f()
    }), new(TestMailboxHandler))

    // 启动发送者（高频发送消息）
    wg.Add(producerNum)
    var idx atomic.Int64
    for i := 0; i < producerNum; i++ {
        go func() {
            defer wg.Done()
            for j := 0; j < totalMessageNum/producerNum; j++ {
                v := idx.Add(1)
                mb.HandleUserMessage(func() {
                    count := n.Add(1)
                    if !addToMap(v) {
                        panic("重复消息执行")
                    }
                    if int(count) == totalMessageNum {
                        select {
                        case <-exitSignal:
                        default:
                            close(exitSignal)
                        }
                    }
                })
            }
        }()
    }

    // 监控 CPU 占用
    globalWait.Add(1)
    go func() {
        var onceDone sync.Once
        defer func() {
            onceDone.Do(func() {
                globalWait.Done()
            })
        }()
        start := time.Now()
        for time.Since(start) < timeout {
            if done.Load() {
                onceDone.Do(func() {
                    globalWait.Done()
                })
                return
            }
            currentN := n.Load()
            if atomic.LoadUint32(&mb.status) == running {
                t.Log("process() 仍在运行... 执行数量：", currentN, "期望数量：", totalMessageNum, "非重复消息数量：", getNotRepeatedNum())
            } else {
                t.Log("process() 已退出，执行数量：", currentN, "期望数量：", totalMessageNum, "非重复消息数量：", getNotRepeatedNum())

                if currentN > int64(totalMessageNum) {
                    t.Error("执行数量大于期望数量")
                    return
                } else {
                    return
                }
            }
            time.Sleep(100 * time.Millisecond)
        }

        close(stopSignal)
    }()

    // 等待测试结束
    select {
    case <-exitSignal:
        finalN := n.Load()
        t.Log("process() 退出，执行数量：", finalN, "期望数量：", totalMessageNum, "非重复消息数量：", getNotRepeatedNum())
    case <-stopSignal:
        t.Fatal("process() 未退出，陷入忙等待")
    case <-time.After(timeout + time.Second):
        // 再次验证数量是否匹配
        finalN := n.Load()
        finalNotRepeated := getNotRepeatedNum()
        t.Log("process() 疑似通过，最终验证：", finalN, "期望数量：", totalMessageNum, "非重复消息数量：", finalNotRepeated)
        if finalN != int64(totalMessageNum) {
            t.Fatal("执行数量不匹配")
        } else {
            if finalN != int64(finalNotRepeated) {
                t.Fatal("存在重复数据")
            }
            t.Log("测试通过")
        }
    }

    done.Store(true)
    globalWait.Wait()
}
