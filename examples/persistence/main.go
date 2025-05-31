package main

import (
    "fmt"
    "time"

    "github.com/kercylan98/vivid/src/persistence/persistencerepos"
    "github.com/kercylan98/vivid/src/vivid"
)

// =============================================================================
// 银行账户示例 - 展示持久化Actor的完整功能
// =============================================================================

// AccountState 账户状态
type AccountState struct {
    AccountID    string    `json:"account_id"`
    Balance      float64   `json:"balance"`
    Transactions []string  `json:"transactions"`
    LastUpdate   time.Time `json:"last_update"`
    RestartCount int       `json:"restart_count"`
}

// Transaction 交易事件
type Transaction struct {
    Type      string    `json:"type"` // "deposit" 或 "withdraw"
    Amount    float64   `json:"amount"`
    Timestamp time.Time `json:"timestamp"`
}

// GetBalanceQuery 查询余额
type GetBalanceQuery struct{}

// GetStateQuery 查询完整状态
type GetStateQuery struct{}

// SimulateErrorCommand 模拟错误
type SimulateErrorCommand struct{}

// BankAccount 银行账户Actor
type BankAccount struct {
    accountID string
    state     *AccountState
}

// NewBankAccount 创建新的银行账户Actor
func NewBankAccount(accountID string) *BankAccount {
    return &BankAccount{
        accountID: accountID,
        state: &AccountState{
            AccountID:    accountID,
            Balance:      0.0,
            Transactions: make([]string, 0),
            LastUpdate:   time.Now(),
            RestartCount: 0,
        },
    }
}

// GetPersistenceId 实现PersistentActor接口
func (b *BankAccount) GetPersistenceId() string {
    return fmt.Sprintf("bank-account-%s", b.accountID)
}

// GetCurrentState 实现PersistentActor接口
func (b *BankAccount) GetCurrentState() any {
    return b.state
}

// RestoreState 实现PersistentActor接口
func (b *BankAccount) RestoreState(state any) {
    if restoredState, ok := state.(*AccountState); ok {
        b.state = restoredState
        fmt.Printf("💾 账户 %s 状态已恢复: 余额=%.2f, 交易记录=%d条, 重启次数=%d\n",
            b.state.AccountID, b.state.Balance, len(b.state.Transactions), b.state.RestartCount)
    }
}

// OnReceive 实现Actor接口
func (b *BankAccount) OnReceive(ctx vivid.ActorContext) {
    switch msg := ctx.Message().(type) {
    case *vivid.OnLaunch:
        if msg.Restarted() {
            b.state.RestartCount++
            b.state.LastUpdate = time.Now()
            fmt.Printf("🔄 账户 %s 已重启 (第%d次)\n", b.state.AccountID, b.state.RestartCount)
        } else {
            fmt.Printf("🚀 账户 %s 已启动\n", b.state.AccountID)
        }

    case *Transaction:
        b.handleTransaction(ctx, msg)

    case *GetBalanceQuery:
        ctx.Reply(b.state.Balance)

    case *GetStateQuery:
        ctx.Reply(b.state)

    case *SimulateErrorCommand:
        fmt.Printf("💥 模拟账户 %s 发生错误\n", b.state.AccountID)
        panic(fmt.Errorf("模拟的账户错误"))
    }
}

// handleTransaction 处理交易
func (b *BankAccount) handleTransaction(ctx vivid.ActorContext, tx *Transaction) {
    // 业务逻辑验证
    if tx.Type == "withdraw" && b.state.Balance < tx.Amount {
        fmt.Printf("❌ 账户 %s 余额不足: 当前余额=%.2f, 尝试提取=%.2f\n", b.state.AccountID, b.state.Balance, tx.Amount)
        ctx.Reply(fmt.Errorf("余额不足"))
        return
    }

    // 持久化交易事件
    if pCtx := ctx.Persistence(); pCtx != nil {
        if err := pCtx.Persist(tx); err == nil {
            // 更新状态
            switch tx.Type {
            case "deposit":
                b.state.Balance += tx.Amount
                fmt.Printf("💰 账户 %s 存款 %.2f，当前余额: %.2f\n", b.state.AccountID, tx.Amount, b.state.Balance)
            case "withdraw":
                b.state.Balance -= tx.Amount
                fmt.Printf("💸 账户 %s 取款 %.2f，当前余额: %.2f\n", b.state.AccountID, tx.Amount, b.state.Balance)
            }

            // 记录交易历史
            txRecord := fmt.Sprintf("%s %.2f at %s", tx.Type, tx.Amount, tx.Timestamp.Format("15:04:05"))
            b.state.Transactions = append(b.state.Transactions, txRecord)
            b.state.LastUpdate = time.Now()

            ctx.Reply("交易成功")
        } else {
            fmt.Printf("❌ 持久化失败: %v\n", err)
            ctx.Reply(fmt.Errorf("持久化失败: %v", err))
        }
    }
}

// =============================================================================
// 主程序
// =============================================================================

func main() {
    fmt.Println("🏦 === 银行账户持久化示例 ===")
    fmt.Println()

    // 创建内存持久化仓库
    repo := persistencerepos.NewMemory()

    // 创建Actor系统
    system := vivid.NewActorSystem().StartP()
    defer system.StopP()

    // 自定义快照策略 - 每3个事件创建快照
    policy := &vivid.AutoSnapshotPolicy{
        EventThreshold:          3,
        TimeThreshold:           30 * time.Second,
        StateChangeThreshold:    0.1, // 状态变化10%时创建快照
        ForceSnapshotOnShutdown: true,
    }

    accountID := "ACC001"

    // 第一阶段：创建账户并进行交易
    fmt.Println("📋 第一阶段：创建账户并进行交易")
    account1 := system.ActorOf(func() vivid.Actor {
        return NewBankAccount(accountID)
    }, func(config *vivid.ActorConfig) {
        config.WithName("bank-account-1").
            WithPersistence(repo, policy).
            WithSupervisor(vivid.SupervisorFN(func(snapshot vivid.AccidentSnapshot) {
                fmt.Printf("🔧 监管者检测到错误，重启账户: %s\n", snapshot.GetReason())
                snapshot.Restart(snapshot.GetVictim())
            }))
    })

    // 执行一系列交易
    transactions := []*Transaction{
        {Type: "deposit", Amount: 1000.0, Timestamp: time.Now()},
        {Type: "deposit", Amount: 500.0, Timestamp: time.Now()},
        {Type: "withdraw", Amount: 200.0, Timestamp: time.Now()},
        {Type: "deposit", Amount: 300.0, Timestamp: time.Now()}, // 应该触发快照
        {Type: "withdraw", Amount: 100.0, Timestamp: time.Now()},
    }

    for i, tx := range transactions {
        future := system.Ask(account1, tx, 1*time.Second)
        result, err := future.Result()
        if err != nil {
            fmt.Printf("交易 %d 失败: %v\n", i+1, err)
        } else {
            fmt.Printf("交易 %d 结果: %v\n", i+1, result)
        }
        time.Sleep(100 * time.Millisecond)
    }

    // 查询当前状态
    future := system.Ask(account1, &GetStateQuery{}, 1*time.Second)
    if result, err := future.Result(); err == nil {
        if state, ok := result.(*AccountState); ok {
            fmt.Printf("\n📊 第一阶段结束状态:\n")
            fmt.Printf("   账户ID: %s\n", state.AccountID)
            fmt.Printf("   余额: %.2f\n", state.Balance)
            fmt.Printf("   交易记录: %d条\n", len(state.Transactions))
            fmt.Printf("   重启次数: %d\n", state.RestartCount)
        }
    }

    // 第二阶段：模拟错误和重启
    fmt.Println("\n🔥 第二阶段：模拟错误和监管重启")
    system.Tell(account1, &SimulateErrorCommand{})
    time.Sleep(500 * time.Millisecond) // 等待重启完成

    // 验证重启后状态
    future = system.Ask(account1, &GetStateQuery{}, 1*time.Second)
    if result, err := future.Result(); err == nil {
        if state, ok := result.(*AccountState); ok {
            fmt.Printf("\n📊 重启后状态:\n")
            fmt.Printf("   账户ID: %s\n", state.AccountID)
            fmt.Printf("   余额: %.2f (应该保持不变)\n", state.Balance)
            fmt.Printf("   交易记录: %d条 (应该保持不变)\n", len(state.Transactions))
            fmt.Printf("   重启次数: %d (应该增加)\n", state.RestartCount)
        }
    }

    // 验证重启后功能正常
    fmt.Println("\n💼 验证重启后功能正常")
    restartTx := &Transaction{Type: "deposit", Amount: 50.0, Timestamp: time.Now()}
    future = system.Ask(account1, restartTx, 1*time.Second)
    if result, err := future.Result(); err == nil {
        fmt.Printf("重启后交易结果: %v\n", result)
    }

    // 关闭第一个账户
    system.PoisonKill(account1, "第一阶段结束")
    time.Sleep(200 * time.Millisecond)

    // 第三阶段：模拟系统重启，创建新的账户实例
    fmt.Println("\n🔄 第三阶段：模拟系统重启，创建新账户实例")
    account2 := system.ActorOf(func() vivid.Actor {
        return NewBankAccount(accountID) // 相同的账户ID
    }, func(config *vivid.ActorConfig) {
        config.WithName("bank-account-2").
            WithPersistence(repo, policy) // 使用相同的仓库
    })

    time.Sleep(200 * time.Millisecond) // 等待恢复完成

    // 验证完整恢复
    future = system.Ask(account2, &GetStateQuery{}, 1*time.Second)
    if result, err := future.Result(); err == nil {
        if state, ok := result.(*AccountState); ok {
            fmt.Printf("\n📊 系统重启后完整恢复状态:\n")
            fmt.Printf("   账户ID: %s\n", state.AccountID)
            fmt.Printf("   余额: %.2f\n", state.Balance)
            fmt.Printf("   交易记录: %d条\n", len(state.Transactions))
            fmt.Printf("   重启次数: %d\n", state.RestartCount)
            fmt.Printf("   最后更新: %s\n", state.LastUpdate.Format("15:04:05"))

            fmt.Println("\n📝 交易历史:")
            for i, tx := range state.Transactions {
                fmt.Printf("   %d. %s\n", i+1, tx)
            }
        }
    }

    // 继续进行交易验证功能完全正常
    fmt.Println("\n✅ 验证恢复后的账户功能")
    finalTx := &Transaction{Type: "withdraw", Amount: 25.0, Timestamp: time.Now()}
    future = system.Ask(account2, finalTx, 1*time.Second)
    if result, err := future.Result(); err == nil {
        fmt.Printf("最终交易结果: %v\n", result)
    }

    // 最终余额查询
    future = system.Ask(account2, &GetBalanceQuery{}, 1*time.Second)
    if result, err := future.Result(); err == nil {
        balance := result.(float64)
        fmt.Printf("\n💰 最终账户余额: %.2f\n", balance)
    }

    fmt.Println("\n🎉 持久化示例完成!")
    fmt.Println("\n✨ 演示功能:")
    fmt.Println("   ✓ 自动事件持久化")
    fmt.Println("   ✓ 智能快照策略")
    fmt.Println("   ✓ 监管策略和自动重启")
    fmt.Println("   ✓ 状态完整恢复")
    fmt.Println("   ✓ 系统重启后数据不丢失")
}
