package main

import (
    "fmt"
    "time"

    "github.com/kercylan98/vivid/src/persistence/persistencerepos"
    "github.com/kercylan98/vivid/src/vivid"
)

// CounterState 计数器状态
type CounterState struct {
    Value   int      `json:"value"`
    History []string `json:"history"`
}

// Increment 增量命令
type Increment struct {
    Amount int    `json:"amount"`
    Note   string `json:"note"`
}

// GetValue 获取当前值
type GetValue struct{}

// Counter 计数器Actor
type Counter struct {
    id    string
    state *CounterState
}

// NewCounter 创建计数器
func NewCounter(id string) *Counter {
    return &Counter{
        id: id,
        state: &CounterState{
            Value:   0,
            History: make([]string, 0),
        },
    }
}

// GetPersistenceId 实现PersistentActor接口
func (c *Counter) GetPersistenceId() string {
    return c.id
}

// GetCurrentState 实现PersistentActor接口
func (c *Counter) GetCurrentState() any {
    return c.state
}

// RestoreState 实现PersistentActor接口
func (c *Counter) RestoreState(state any) {
    if restoredState, ok := state.(*CounterState); ok {
        c.state = restoredState
        fmt.Printf("📄 计数器 %s 状态已恢复: 值=%d\n", c.id, c.state.Value)
    }
}

// OnReceive 实现Actor接口
func (c *Counter) OnReceive(ctx vivid.ActorContext) {
    switch msg := ctx.Message().(type) {
    case *vivid.OnLaunch:
        fmt.Printf("🚀 计数器 %s 已启动\n", c.id)

    case *Increment:
        // 持久化增量事件
        if pCtx := ctx.Persistence(); pCtx != nil {
            if err := pCtx.Persist(msg); err == nil {
                // 更新状态
                c.state.Value += msg.Amount
                c.state.History = append(c.state.History,
                    fmt.Sprintf("+%d (%s)", msg.Amount, msg.Note))

                fmt.Printf("➕ 计数器 %s: %d -> %d\n",
                    c.id, c.state.Value-msg.Amount, c.state.Value)

                ctx.Reply(c.state.Value)
            } else {
                fmt.Printf("❌ 持久化失败: %v\n", err)
                ctx.Reply(err)
            }
        }

    case *GetValue:
        ctx.Reply(c.state)
    }
}

func main() {
    fmt.Println("🔢 === 基础持久化示例 ===\n")

    // 创建持久化仓库
    repo := persistencerepos.NewMemory()

    // 创建Actor系统
    system := vivid.NewActorSystem().StartP()
    defer system.StopP()

    counterID := "my-counter"

    fmt.Println("第一轮：创建计数器并增加值")
    // 创建第一个计数器实例
    counter1 := system.ActorOf(func() vivid.Actor {
        return NewCounter(counterID)
    }, func(config *vivid.ActorConfig) {
        config.WithName("counter-1").WithDefaultPersistence(repo)
    })

    // 进行一些增量操作
    increments := []*Increment{
        {Amount: 5, Note: "初始值"},
        {Amount: 3, Note: "第一次增加"},
        {Amount: 7, Note: "第二次增加"},
    }

    for _, inc := range increments {
        future := system.Ask(counter1, inc, 1*time.Second)
        if result, err := future.Result(); err == nil {
            fmt.Printf("当前值: %v\n", result)
        }
        time.Sleep(100 * time.Millisecond)
    }

    // 获取当前完整状态
    future := system.Ask(counter1, &GetValue{}, 1*time.Second)
    if result, err := future.Result(); err == nil {
        if state, ok := result.(*CounterState); ok {
            fmt.Printf("\n📊 第一轮结束状态:\n")
            fmt.Printf("   值: %d\n", state.Value)
            fmt.Printf("   历史记录: %v\n", state.History)
        }
    }

    // 关闭第一个计数器
    system.PoisonKill(counter1, "第一轮结束")
    time.Sleep(100 * time.Millisecond)

    fmt.Println("\n第二轮：重新创建计数器（相同ID）")
    // 创建第二个计数器实例（相同ID，应该恢复状态）
    counter2 := system.ActorOf(func() vivid.Actor {
        return NewCounter(counterID) // 相同的ID
    }, func(config *vivid.ActorConfig) {
        config.WithName("counter-2").WithDefaultPersistence(repo)
    })

    time.Sleep(100 * time.Millisecond) // 等待恢复完成

    // 验证状态已恢复
    future = system.Ask(counter2, &GetValue{}, 1*time.Second)
    if result, err := future.Result(); err == nil {
        if state, ok := result.(*CounterState); ok {
            fmt.Printf("\n📊 恢复后状态:\n")
            fmt.Printf("   值: %d (应该是15)\n", state.Value)
            fmt.Printf("   历史记录: %v\n", state.History)
        }
    }

    // 继续增加值，证明功能正常
    fmt.Println("\n继续操作验证功能正常:")
    future = system.Ask(counter2, &Increment{Amount: 2, Note: "恢复后增加"}, 1*time.Second)
    if result, err := future.Result(); err == nil {
        fmt.Printf("最终值: %v\n", result)
    }

    fmt.Println("\n✅ 基础持久化示例完成!")
    fmt.Println("✨ 演示了:")
    fmt.Println("   • 事件持久化")
    fmt.Println("   • 状态自动恢复")
    fmt.Println("   • 简单易用的API")
}
