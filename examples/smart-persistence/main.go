package main

import (
	"fmt"
	"time"

	"github.com/kercylan98/vivid/src/persistence/persistencerepos"
	"github.com/kercylan98/vivid/src/vivid"
)

// BankAccountState 银行账户状态
type BankAccountState struct {
	AccountID string  `json:"account_id"`
	Balance   float64 `json:"balance"`
	Version   int64   `json:"version"`
}

// DepositEvent 存款事件
type DepositEvent struct {
	Amount float64 `json:"amount"`
	Time   int64   `json:"time"`
}

// WithdrawEvent 取款事件
type WithdrawEvent struct {
	Amount float64 `json:"amount"`
	Time   int64   `json:"time"`
}

// SmartBankAccountActor 智能银行账户Actor
type SmartBankAccountActor struct {
	state *BankAccountState
}

func NewSmartBankAccountActor(accountID string) *SmartBankAccountActor {
	return &SmartBankAccountActor{
		state: &BankAccountState{
			AccountID: accountID,
			Balance:   0.0,
			Version:   0,
		},
	}
}

// GetPersistenceId 实现SmartPersistentActor接口
func (a *SmartBankAccountActor) GetPersistenceId() string {
	return fmt.Sprintf("bank-account-%s", a.state.AccountID)
}

// GetCurrentState 实现SmartPersistentActor接口 - 用于自动快照
func (a *SmartBankAccountActor) GetCurrentState() any {
	return a.state
}

// ApplyEvent 实现SmartPersistentActor接口 - 用于事件重放
func (a *SmartBankAccountActor) ApplyEvent(event any) {
	switch e := event.(type) {
	case *DepositEvent:
		a.state.Balance += e.Amount
		a.state.Version++
		fmt.Printf("[恢复] 应用存款事件: +%.2f, 余额: %.2f\n", e.Amount, a.state.Balance)

	case *WithdrawEvent:
		a.state.Balance -= e.Amount
		a.state.Version++
		fmt.Printf("[恢复] 应用取款事件: -%.2f, 余额: %.2f\n", e.Amount, a.state.Balance)
	}
}

// OnRecover 实现SmartPersistentActor接口
func (a *SmartBankAccountActor) OnRecover(ctx vivid.SmartPersistenceContext) {
	fmt.Printf("=== 开始恢复账户 %s ===\n", a.state.AccountID)

	// 从快照恢复
	if snapshot := ctx.GetSnapshot(); snapshot != nil {
		if state, ok := snapshot.(*BankAccountState); ok {
			a.state = state
			fmt.Printf("[恢复] 从快照恢复: 余额=%.2f, 版本=%d\n", a.state.Balance, a.state.Version)
		}
	}

	// 重放事件
	events := ctx.GetEvents()
	if len(events) > 0 {
		fmt.Printf("[恢复] 开始重放 %d 个事件\n", len(events))
		for _, event := range events {
			a.ApplyEvent(event)
		}
	}

	fmt.Printf("=== 恢复完成，最终余额: %.2f ===\n", a.state.Balance)
}

// OnReceive 实现Actor接口
func (a *SmartBankAccountActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *DepositEvent:
		// 先更新状态
		a.state.Balance += msg.Amount
		a.state.Version++

		// 然后持久化事件和当前状态
		if smartCtx := ctx.Persistence().(vivid.SmartPersistenceContext); smartCtx != nil {
			err := smartCtx.PersistWithState(msg, a.GetCurrentState())
			if err != nil {
				fmt.Printf("持久化失败: %v\n", err)
				// 回滚状态
				a.state.Balance -= msg.Amount
				a.state.Version--
				return
			}
		}

		fmt.Printf("存款成功: +%.2f, 当前余额: %.2f (事件计数: %d)\n",
			msg.Amount, a.state.Balance, ctx.Persistence().(vivid.SmartPersistenceContext).GetEventCount())
		ctx.Reply(a.state.Balance)

	case *WithdrawEvent:
		// 检查余额
		if a.state.Balance < msg.Amount {
			ctx.Reply(fmt.Errorf("余额不足: 当前%.2f, 需要%.2f", a.state.Balance, msg.Amount))
			return
		}

		// 先更新状态
		a.state.Balance -= msg.Amount
		a.state.Version++

		// 然后持久化事件和当前状态
		if smartCtx := ctx.Persistence().(vivid.SmartPersistenceContext); smartCtx != nil {
			err := smartCtx.PersistWithState(msg, a.GetCurrentState())
			if err != nil {
				fmt.Printf("持久化失败: %v\n", err)
				// 回滚状态
				a.state.Balance += msg.Amount
				a.state.Version--
				return
			}
		}

		fmt.Printf("取款成功: -%.2f, 当前余额: %.2f (事件计数: %d)\n",
			msg.Amount, a.state.Balance, ctx.Persistence().(vivid.SmartPersistenceContext).GetEventCount())
		ctx.Reply(a.state.Balance)

	case string:
		switch msg {
		case "balance":
			ctx.Reply(a.state.Balance)
		case "force_snapshot":
			// 手动强制快照
			if smartCtx := ctx.Persistence().(vivid.SmartPersistenceContext); smartCtx != nil {
				err := smartCtx.ForceSnapshot(a.GetCurrentState())
				if err != nil {
					fmt.Printf("强制快照失败: %v\n", err)
				} else {
					fmt.Printf("强制快照成功，当前余额: %.2f\n", a.state.Balance)
				}
			}
			ctx.Reply("snapshot_created")
		case "status":
			policy := ctx.Persistence().(vivid.SmartPersistenceContext).GetSnapshotPolicy()
			fmt.Printf("账户状态: 余额=%.2f, 版本=%d\n", a.state.Balance, a.state.Version)
			fmt.Printf("快照策略: 事件阈值=%d, 时间阈值=%v\n",
				policy.EventThreshold, policy.TimeThreshold)
			fmt.Printf("当前事件计数: %d\n",
				ctx.Persistence().(vivid.SmartPersistenceContext).GetEventCount())
			ctx.Reply("ok")
		}
	}
}

func main() {
	fmt.Println("=== 智能持久化银行账户示例 ===")

	// 创建内存持久化仓库
	repository := persistencerepos.NewMemory()

	// 创建Actor系统
	system := vivid.NewActorSystem()
	system.StartP()
	defer system.StopP()

	// 创建自定义快照策略 - 每5个事件或每30秒创建快照
	customPolicy := &vivid.AutoSnapshotPolicy{
		EventThreshold:          5,
		TimeThreshold:           30 * time.Second,
		StateChangeThreshold:    0.2,
		ForceSnapshotOnShutdown: true,
	}

	// 第一阶段：创建账户并进行一些操作
	fmt.Println("\n--- 第一阶段：创建新账户 ---")
	account1 := system.ActorOf(func() vivid.Actor {
		return NewSmartBankAccountActor("ACC001")
	}, func(config *vivid.ActorConfig) {
		config.WithName("smart-bank-account-1").
			WithSmartPersistence(repository, customPolicy, nil)
	})

	// 进行一系列操作
	operations := []struct {
		desc   string
		action func()
	}{
		{"存款100元", func() {
			future := system.Ask(account1, &DepositEvent{Amount: 100.0, Time: time.Now().Unix()}, time.Second)
			result, _ := future.Result()
			fmt.Printf("操作结果: %.2f\n", result)
		}},
		{"存款50元", func() {
			future := system.Ask(account1, &DepositEvent{Amount: 50.0, Time: time.Now().Unix()}, time.Second)
			result, _ := future.Result()
			fmt.Printf("操作结果: %.2f\n", result)
		}},
		{"取款30元", func() {
			future := system.Ask(account1, &WithdrawEvent{Amount: 30.0, Time: time.Now().Unix()}, time.Second)
			result, _ := future.Result()
			fmt.Printf("操作结果: %.2f\n", result)
		}},
		{"查看状态", func() {
			future := system.Ask(account1, "status", time.Second)
			future.Result()
		}},
		{"存款20元", func() {
			future := system.Ask(account1, &DepositEvent{Amount: 20.0, Time: time.Now().Unix()}, time.Second)
			result, _ := future.Result()
			fmt.Printf("操作结果: %.2f\n", result)
		}},
		{"存款15元", func() {
			future := system.Ask(account1, &DepositEvent{Amount: 15.0, Time: time.Now().Unix()}, time.Second)
			result, _ := future.Result()
			fmt.Printf("操作结果: %.2f (自动快照应该被触发)\n", result)
		}},
		{"取款25元", func() {
			future := system.Ask(account1, &WithdrawEvent{Amount: 25.0, Time: time.Now().Unix()}, time.Second)
			result, _ := future.Result()
			fmt.Printf("操作结果: %.2f\n", result)
		}},
		{"存款10元", func() {
			future := system.Ask(account1, &DepositEvent{Amount: 10.0, Time: time.Now().Unix()}, time.Second)
			result, _ := future.Result()
			fmt.Printf("操作结果: %.2f\n", result)
		}},
	}

	for _, op := range operations {
		fmt.Printf("\n执行操作: %s\n", op.desc)
		op.action()
		time.Sleep(100 * time.Millisecond) // 等待操作完成
	}

	// 等待一段时间确保所有操作完成
	time.Sleep(500 * time.Millisecond)

	// 第二阶段：重新创建相同账户，验证恢复功能
	fmt.Println("\n--- 第二阶段：模拟系统重启，恢复账户 ---")
	account2 := system.ActorOf(func() vivid.Actor {
		return NewSmartBankAccountActor("ACC001") // 相同的账户ID
	}, func(config *vivid.ActorConfig) {
		config.WithName("smart-bank-account-2").
			WithSmartPersistence(repository, customPolicy, nil)
	})

	// 等待恢复完成
	time.Sleep(200 * time.Millisecond)

	// 验证恢复后的余额
	fmt.Println("\n验证恢复后的状态:")
	future := system.Ask(account2, "balance", time.Second)
	balance, _ := future.Result()
	fmt.Printf("恢复后的余额: %.2f\n", balance)

	// 继续进行一些操作验证功能正常
	fmt.Println("\n继续进行操作验证:")
	future = system.Ask(account2, &DepositEvent{Amount: 35.0, Time: time.Now().Unix()}, time.Second)
	result, _ := future.Result()
	fmt.Printf("追加存款35元后余额: %.2f\n", result)

	// 展示智能持久化的优势
	fmt.Println("\n=== 智能持久化特性展示 ===")
	fmt.Println("✓ 自动快照管理 - 根据事件数量和时间策略")
	fmt.Println("✓ 自动深拷贝 - 避免状态引用问题")
	fmt.Println("✓ 策略可配置 - 支持自定义快照策略")
	fmt.Println("✓ 智能恢复 - 快照+事件重放")
	fmt.Println("✓ 零用户管理 - 无需手动管理快照时机")

	fmt.Println("\n示例完成！")
}
