// Package main 演示了 Vivid Actor 系统的监控和自定义指标功能。
// 该示例展示了如何使用计数器、计量器、直方图和时间指标来监控银行业务操作。
package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/kercylan98/vivid/src/vivid"
)

// DepositMessage 表示存款操作的消息。
type DepositMessage struct {
	Amount int64
}

// WithdrawMessage 表示取款操作的消息。
type WithdrawMessage struct {
	Amount int64
}

// BalanceMessage 表示余额查询的消息。
type BalanceMessage struct{}

// BalanceResponse 表示余额查询的回复。
type BalanceResponse struct {
	Balance int64
}

// TransferMessage 表示转账操作的消息。
type TransferMessage struct {
	To     vivid.ActorRef
	Amount int64
}

// BankAccount 实现了银行账户 Actor，用于演示监控功能。
type BankAccount struct {
	id      string
	balance int64
}

// NewBankAccount 创建一个新的银行账户实例。
func NewBankAccount(id string) *BankAccount {
	return &BankAccount{
		id:      id,
		balance: 1000, // 初始余额1000
	}
}

func (ba *BankAccount) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		fmt.Printf("[%s] 银行账户已启动，初始余额: %d\n", ba.id, ba.balance)

	case *DepositMessage:
		ba.balance += msg.Amount
		fmt.Printf("[%s] 存款 %d，当前余额: %d\n", ba.id, msg.Amount, ba.balance)
		// 不自动回复，让调用者决定是否需要回复

	case *WithdrawMessage:
		if ba.balance >= msg.Amount {
			ba.balance -= msg.Amount
			fmt.Printf("[%s] 取款 %d，当前余额: %d\n", ba.id, msg.Amount, ba.balance)
		} else {
			fmt.Printf("[%s] 取款失败，余额不足。当前余额: %d，尝试取款: %d\n", ba.id, ba.balance, msg.Amount)
			// 模拟错误情况
			panic("余额不足")
		}

	case *BalanceMessage:
		// 检查是否有发送者，只有Ask调用才有发送者，Tell调用没有
		if ctx.Sender() != nil {
			// 只有在Ask调用时才回复
			ctx.Reply(&BalanceResponse{Balance: ba.balance})
		}
		// Tell调用时不回复，避免警告

	case *TransferMessage:
		if ba.balance >= msg.Amount {
			ba.balance -= msg.Amount
			// 向目标账户发送存款消息 - 使用ctx.Tell
			ctx.Tell(msg.To, &DepositMessage{Amount: msg.Amount})
			fmt.Printf("[%s] 转账 %d 到目标账户，当前余额: %d\n", ba.id, msg.Amount, ba.balance)
		} else {
			fmt.Printf("[%s] 转账失败，余额不足。当前余额: %d，尝试转账: %d\n", ba.id, ba.balance, msg.Amount)
		}

	case *vivid.OnKill:
		// 内部系统消息，静默处理
		fmt.Printf("[%s] 正在关闭...\n", ba.id)

	default:
		// 只记录非系统消息的未知类型
		if !isSystemMessage(msg) {
			fmt.Printf("[%s] 收到未知消息类型: %T\n", ba.id, msg)
		}
	}
}

// isSystemMessage 检查是否为系统消息
func isSystemMessage(msg interface{}) bool {
	switch msg.(type) {
	case *vivid.OnKill, vivid.OnKill:
		return true
	default:
		return false
	}
}

// MonitoringActor 监控Actor，用于接收和展示监控事件
type MonitoringActor struct{}

func (ma *MonitoringActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.MetricsSnapshot:
		fmt.Printf("\n=== 📊 系统指标快照 ===\n")
		fmt.Printf("时间戳: %s\n", msg.Timestamp.Format("15:04:05"))
		fmt.Printf("已发送消息: %d\n", msg.MessagesSent)
		fmt.Printf("已接收消息: %d\n", msg.MessagesReceived)
		fmt.Printf("消息吞吐量: %.2f msg/s\n", msg.MessageThroughput)
		fmt.Printf("平均延迟: %v\n", msg.AverageLatency)
		fmt.Printf("活跃Actor: %d\n", msg.ActiveActors)
		fmt.Printf("错误计数: %d\n", msg.ErrorCount)
		fmt.Printf("错误率: %.2f%%\n", msg.ErrorRate)
		fmt.Printf("协程数量: %d\n", runtime.NumGoroutine())
		fmt.Printf("========================\n\n")

	case map[string]interface{}:
		// 系统指标事件
		if timestamp, ok := msg["timestamp"]; ok {
			fmt.Printf("🔧 系统指标更新: %v\n", timestamp)
			if memUsage, ok := msg["memory_usage"]; ok {
				fmt.Printf("  内存使用: %d bytes\n", memUsage)
			}
			if gcCount, ok := msg["gc_count"]; ok {
				fmt.Printf("  GC次数: %d\n", gcCount)
			}
		}

	case *vivid.OnKill:
		// 内部系统消息，静默处理
		fmt.Printf("监控Actor正在关闭...\n")

	default:
		// 只记录非系统消息的未知类型
		if !isSystemMessage(msg) {
			fmt.Printf("监控Actor收到消息: %T\n", msg)
		}
	}
}

// ControllerActor 控制器Actor，负责发送消息到银行账户
type ControllerActor struct {
	accounts      []vivid.ActorRef
	totalMessages int
	metrics       vivid.Metrics
}

func NewControllerActor(accounts []vivid.ActorRef, totalMessages int, metrics vivid.Metrics) *ControllerActor {
	return &ControllerActor{
		accounts:      accounts,
		totalMessages: totalMessages,
		metrics:       metrics,
	}
}

func (c *ControllerActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		fmt.Printf("🚀 控制器Actor启动，开始发送%d条消息\n", c.totalMessages)

		// 发送所有消息并记录业务指标
		for i := 0; i < c.totalMessages; i++ {
			// 选择目标账户
			account := c.accounts[i%len(c.accounts)]

			// 根据比例分配不同类型的操作
			switch {
			case i%20 == 0 && len(c.accounts) > 1:
				// 5% 转账操作
				fromAccount := c.accounts[i%len(c.accounts)]
				toAccount := c.accounts[(i+1)%len(c.accounts)]
				ctx.Tell(fromAccount, &TransferMessage{To: toAccount, Amount: 50})

				// 记录业务指标：转账操作
				if c.metrics != nil {
					c.metrics.IncrementCounter("total_transactions", 1)
					c.metrics.IncrementCounter("transfer_transactions", 1)
					c.metrics.RecordTiming("transfer_duration", time.Duration(5+i%10)*time.Millisecond)

					// 记录转账金额分布
					c.metrics.RecordHistogram("transfer_amount", 50.0)

					// 更新平均转账额度指标
					c.metrics.SetGauge("avg_transfer_amount", 50.0)

					// 记录高价值转账的特殊计数器
					if 50 > 100 {
						c.metrics.IncrementCounter("high_value_transfers", 1)
					}
				}

			case i%50 == 0:
				// 2% 余额查询
				ctx.Tell(account, &BalanceMessage{})

				// 记录业务指标：查询操作
				if c.metrics != nil {
					c.metrics.IncrementCounter("total_transactions", 1)
					c.metrics.IncrementCounter("query_transactions", 1)

					// 记录查询响应时间
					queryLatency := time.Duration(1+i%3) * time.Millisecond
					c.metrics.RecordTiming("query_response_time", queryLatency)

					// 记录查询频率分布
					c.metrics.RecordHistogram("query_frequency", float64((i/50)%10+1))

					// 更新当前查询负载
					c.metrics.SetGauge("current_query_load", float64(i%5))

					// 模拟少量失败的查询
					if i%100 == 0 {
						c.metrics.IncrementCounter("failed_transactions", 1)
						c.metrics.IncrementCounter("failed_queries", 1)

						// 记录失败查询的延迟分布
						c.metrics.RecordHistogram("failed_query_latency", float64(queryLatency.Milliseconds()))
					} else {
						c.metrics.IncrementCounter("successful_queries", 1)
					}
				}

			default:
				// 93% 存款操作
				amount := int64((i + 1) * 10)
				ctx.Tell(account, &DepositMessage{Amount: amount})

				// 记录业务指标：存款操作
				if c.metrics != nil {
					c.metrics.IncrementCounter("total_transactions", 1)
					c.metrics.IncrementCounter("deposit_transactions", 1)

					// 记录存款处理时间
					depositLatency := time.Duration(2+i%5) * time.Millisecond
					c.metrics.RecordTiming("deposit_duration", depositLatency)

					// 记录交易金额分布（直方图）
					c.metrics.RecordHistogram("transaction_amount", float64(amount))

					// 记录不同金额范围的存款
					if amount < 50 {
						c.metrics.IncrementCounter("small_deposits", 1)
					} else if amount < 200 {
						c.metrics.IncrementCounter("medium_deposits", 1)
					} else {
						c.metrics.IncrementCounter("large_deposits", 1)
					}

					// 更新当前平均存款金额
					currentAvg := float64(c.metrics.GetCounter("deposit_transactions"))
					if currentAvg > 0 {
						c.metrics.SetGauge("avg_deposit_amount", float64(amount))
					}

					// 记录峰值存款金额
					currentMax := c.metrics.GetGauge("max_deposit_amount")
					if float64(amount) > currentMax {
						c.metrics.SetGauge("max_deposit_amount", float64(amount))
					}
				}
			}
		}

		fmt.Printf("✅ 控制器Actor已发送所有%d条消息\n", c.totalMessages)

		// 记录在线用户数变化（模拟业务指标）
		if c.metrics != nil {
			// 模拟用户活动增长
			currentUsers := 1000 + float64(c.totalMessages)/20
			c.metrics.SetGauge("online_users", currentUsers)
			c.metrics.SetGauge("daily_active_users", currentUsers*0.8)
		}

	case *vivid.OnKill:
		fmt.Printf("[控制器] 正在关闭...\n")

	default:
		if !isSystemMessage(msg) {
			fmt.Printf("[控制器] 收到未知消息: %T\n", msg)
		}
	}
}

func main() {
	fmt.Println("🎬 启动Actor监控系统演示...")

	// 使用With函数链式配置监控
	monitoringConfig := vivid.DefaultMonitoringConfig().
		WithMetricsInterval(time.Millisecond * 500).
		WithMaxActorMetrics(1000).
		WithCustomMetricsExporter(func(system vivid.SystemMetricsSnapshot, custom vivid.CustomMetricsSnapshot) {
			// 自定义指标导出器可以将数据发送到外部监控系统
			// 例如：Prometheus、InfluxDB、CloudWatch等
		})

	// 创建指标收集器 - 全局共享的监控实例
	metrics := vivid.NewMetricsCollector(monitoringConfig)

	// 创建Actor系统
	system := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFN(func(config *vivid.ActorSystemConfig) {
		// 配置全局监控，系统级消息会自动被监控（用户不需要关心）
		config.WithGlobalMonitoring(metrics)
	}))
	system.StartP()

	// 优雅停止逻辑
	defer func() {
		fmt.Println("⏰ 正在停止Actor系统...")
		system.PoisonStopP()
		fmt.Println("⏰ Actor系统已停止")

		// 显示最终系统指标
		systemSnapshot := metrics.GetSystemSnapshot()
		fmt.Printf("\n📋 系统停止后最终状态:\n")
		fmt.Printf("==================\n")
		fmt.Printf("📊 系统指标:\n")
		fmt.Printf("- 总发送消息: %d\n", systemSnapshot.MessagesSent)
		fmt.Printf("- 总接收消息: %d\n", systemSnapshot.MessagesReceived)
		fmt.Printf("- 消息计数差异: %d (发送-接收)\n", systemSnapshot.MessagesSent-systemSnapshot.MessagesReceived)
		fmt.Printf("- 平均延迟: %v\n", systemSnapshot.AverageLatency)
		fmt.Printf("- 运行时间: %v\n", systemSnapshot.UptimeDuration.Round(time.Millisecond))
		fmt.Printf("- 活跃Actor: %d\n", systemSnapshot.ActiveActors)
		fmt.Printf("==================\n")

		// 显示业务指标
		fmt.Printf("\n💼 业务指标:\n")
		fmt.Printf("- 在线用户数: %.0f\n", metrics.GetGauge("online_users"))
		fmt.Printf("- 总交易次数: %d\n", metrics.GetCounter("total_transactions"))
		fmt.Printf("- 失败交易数: %d\n", metrics.GetCounter("failed_transactions"))

		// 显示详细的交易分类统计
		fmt.Printf("\n📊 交易分类统计:\n")
		fmt.Printf("- 存款交易: %d\n", metrics.GetCounter("deposit_transactions"))
		fmt.Printf("- 转账交易: %d\n", metrics.GetCounter("transfer_transactions"))
		fmt.Printf("- 查询交易: %d\n", metrics.GetCounter("query_transactions"))
		fmt.Printf("- 成功查询: %d\n", metrics.GetCounter("successful_queries"))
		fmt.Printf("- 失败查询: %d\n", metrics.GetCounter("failed_queries"))

		// 显示存款金额分析
		fmt.Printf("\n💰 存款金额分析:\n")
		fmt.Printf("- 小额存款 (<50): %d\n", metrics.GetCounter("small_deposits"))
		fmt.Printf("- 中额存款 (50-200): %d\n", metrics.GetCounter("medium_deposits"))
		fmt.Printf("- 大额存款 (>200): %d\n", metrics.GetCounter("large_deposits"))
		fmt.Printf("- 平均存款金额: %.2f\n", metrics.GetGauge("avg_deposit_amount"))
		fmt.Printf("- 最大存款金额: %.2f\n", metrics.GetGauge("max_deposit_amount"))

		// 显示时间指标
		fmt.Printf("\n⏱️ 性能指标:\n")
		if transferTiming := metrics.GetTiming("transfer_duration"); transferTiming != nil {
			fmt.Printf("- 转账平均时长: %v (共%d次)\n", transferTiming.Mean, transferTiming.Count)
			fmt.Printf("- 转账最长时长: %v\n", transferTiming.Max)
			fmt.Printf("- 转账最短时长: %v\n", transferTiming.Min)
		}

		if depositTiming := metrics.GetTiming("deposit_duration"); depositTiming != nil {
			fmt.Printf("- 存款平均时长: %v (共%d次)\n", depositTiming.Mean, depositTiming.Count)
			fmt.Printf("- 存款最长时长: %v\n", depositTiming.Max)
		}

		if queryTiming := metrics.GetTiming("query_response_time"); queryTiming != nil {
			fmt.Printf("- 查询平均响应时间: %v (共%d次)\n", queryTiming.Mean, queryTiming.Count)
		}

		// 显示直方图数据
		fmt.Printf("\n📈 分布分析:\n")
		if amountHist := metrics.GetHistogram("transaction_amount"); amountHist != nil {
			fmt.Printf("- 交易金额分布: 均值=%.2f, 最小=%.2f, 最大=%.2f (共%d笔)\n",
				amountHist.Mean, amountHist.Min, amountHist.Max, amountHist.Count)
		}

		if queryFreqHist := metrics.GetHistogram("query_frequency"); queryFreqHist != nil {
			fmt.Printf("- 查询频率分布: 均值=%.2f, 最大=%.2f (共%d次)\n",
				queryFreqHist.Mean, queryFreqHist.Max, queryFreqHist.Count)
		}

		// 显示实时指标
		fmt.Printf("\n🔄 实时指标:\n")
		fmt.Printf("- 当前查询负载: %.1f\n", metrics.GetGauge("current_query_load"))
		fmt.Printf("- 平均转账金额: %.2f\n", metrics.GetGauge("avg_transfer_amount"))
		fmt.Printf("==================\n")

		if systemSnapshot.MessagesSent == systemSnapshot.MessagesReceived {
			fmt.Printf("\n✅ 消息计数完全匹配！监控系统运行正常。\n")
		} else {
			fmt.Printf("\n🔍 存在 %d 条消息差异，这在异步系统中是正常的。\n",
				systemSnapshot.MessagesSent-systemSnapshot.MessagesReceived)
		}
	}()

	// 创建监控Actor（现在用于演示业务指标）
	monitoringActor := system.ActorOf(func() vivid.Actor {
		return &MonitoringActor{}
	}, func(config *vivid.ActorConfig) {
		config.WithName("monitoring-actor").
			WithMonitoring(metrics)
	})

	// 创建银行账户Actor
	fmt.Println("🏦 创建银行账户...")
	accounts := make([]vivid.ActorRef, 5)
	for i := 0; i < 5; i++ {
		accounts[i] = system.ActorOf(func() vivid.Actor {
			return NewBankAccount(fmt.Sprintf("账户%03d", i+1))
		}, func(config *vivid.ActorConfig) {
			config.WithName(fmt.Sprintf("account-%03d", i+1)).
				WithMonitoring(metrics)
		})
	}

	fmt.Printf("✅ 创建了%d个账户 + 1个监控Actor\n", len(accounts))

	// 模拟在线用户数变化
	metrics.SetGauge("online_users", 1000)

	// 延迟启动
	time.Sleep(time.Millisecond * 100)

	fmt.Println("🎬 执行银行业务操作演示...")

	// 简化的业务操作演示：1000条消息
	totalMessages := 1000

	fmt.Printf("🚀 创建控制器Actor，发送%d条消息...\n", totalMessages)

	startTime := time.Now()

	// 创建控制器Actor
	controllerActor := system.ActorOf(func() vivid.Actor {
		return NewControllerActor(accounts, totalMessages, metrics) // 传入metrics用于业务指标
	}, func(config *vivid.ActorConfig) {
		config.WithName("controller").
			WithMonitoring(metrics)
	})

	_ = controllerActor
	_ = monitoringActor

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	fmt.Printf("✅ 控制器Actor创建完成，耗时: %v\n", duration)

	// 等待消息处理完成
	fmt.Println("⏰ 等待所有消息处理完成...")

	// 智能等待
	previousReceived := int64(0)
	stableCount := 0
	maxWaitTime := time.Second * 5
	waitStartTime := time.Now()

	for time.Since(waitStartTime) < maxWaitTime {
		time.Sleep(time.Millisecond * 200)

		currentSnapshot := metrics.GetSystemSnapshot()

		if currentSnapshot.MessagesReceived == previousReceived {
			stableCount++
			if stableCount >= 5 {
				fmt.Printf("💫 消息处理趋于稳定，接收消息数: %d\n", currentSnapshot.MessagesReceived)
				break
			}
		} else {
			stableCount = 0
			previousReceived = currentSnapshot.MessagesReceived
		}
	}

	fmt.Printf("⏳ 消息处理等待完成，耗时: %v\n", time.Since(waitStartTime).Round(time.Millisecond))

	// 获取最终统计
	finalSnapshot := metrics.GetSystemSnapshot()
	fmt.Printf("\n🎯 业务操作演示完成！\n")
	fmt.Printf("==================\n")
	fmt.Printf("📊 系统性能指标:\n")
	fmt.Printf("- 总发送消息: %d\n", finalSnapshot.MessagesSent)
	fmt.Printf("- 总接收消息: %d\n", finalSnapshot.MessagesReceived)
	fmt.Printf("- 消息计数差异: %d (发送-接收)\n", finalSnapshot.MessagesSent-finalSnapshot.MessagesReceived)
	fmt.Printf("- 运行时间: %v\n", finalSnapshot.UptimeDuration.Round(time.Millisecond))
	fmt.Printf("- 平均吞吐量: %.2f msg/s\n", finalSnapshot.MessageThroughput)
	fmt.Printf("- 平均延迟: %v\n", finalSnapshot.AverageLatency)
	fmt.Printf("- 活跃Actor: %d\n", finalSnapshot.ActiveActors)
	fmt.Printf("==================\n")

	// 显示业务指标
	fmt.Printf("\n💼 业务指标:\n")
	fmt.Printf("- 在线用户数: %.0f\n", metrics.GetGauge("online_users"))
	fmt.Printf("- 总交易次数: %d\n", metrics.GetCounter("total_transactions"))
	fmt.Printf("- 失败交易数: %d\n", metrics.GetCounter("failed_transactions"))

	// 显示详细的交易分类统计
	fmt.Printf("\n📊 交易分类统计:\n")
	fmt.Printf("- 存款交易: %d\n", metrics.GetCounter("deposit_transactions"))
	fmt.Printf("- 转账交易: %d\n", metrics.GetCounter("transfer_transactions"))
	fmt.Printf("- 查询交易: %d\n", metrics.GetCounter("query_transactions"))
	fmt.Printf("- 成功查询: %d\n", metrics.GetCounter("successful_queries"))
	fmt.Printf("- 失败查询: %d\n", metrics.GetCounter("failed_queries"))

	// 显示存款金额分析
	fmt.Printf("\n💰 存款金额分析:\n")
	fmt.Printf("- 小额存款 (<50): %d\n", metrics.GetCounter("small_deposits"))
	fmt.Printf("- 中额存款 (50-200): %d\n", metrics.GetCounter("medium_deposits"))
	fmt.Printf("- 大额存款 (>200): %d\n", metrics.GetCounter("large_deposits"))
	fmt.Printf("- 平均存款金额: %.2f\n", metrics.GetGauge("avg_deposit_amount"))
	fmt.Printf("- 最大存款金额: %.2f\n", metrics.GetGauge("max_deposit_amount"))

	// 显示时间指标
	fmt.Printf("\n⏱️ 性能指标:\n")
	if transferTiming := metrics.GetTiming("transfer_duration"); transferTiming != nil {
		fmt.Printf("- 转账平均时长: %v (共%d次)\n", transferTiming.Mean, transferTiming.Count)
		fmt.Printf("- 转账最长时长: %v\n", transferTiming.Max)
		fmt.Printf("- 转账最短时长: %v\n", transferTiming.Min)
	}

	if depositTiming := metrics.GetTiming("deposit_duration"); depositTiming != nil {
		fmt.Printf("- 存款平均时长: %v (共%d次)\n", depositTiming.Mean, depositTiming.Count)
		fmt.Printf("- 存款最长时长: %v\n", depositTiming.Max)
	}

	if queryTiming := metrics.GetTiming("query_response_time"); queryTiming != nil {
		fmt.Printf("- 查询平均响应时间: %v (共%d次)\n", queryTiming.Mean, queryTiming.Count)
	}

	// 显示直方图数据
	fmt.Printf("\n📈 分布分析:\n")
	if amountHist := metrics.GetHistogram("transaction_amount"); amountHist != nil {
		fmt.Printf("- 交易金额分布: 均值=%.2f, 最小=%.2f, 最大=%.2f (共%d笔)\n",
			amountHist.Mean, amountHist.Min, amountHist.Max, amountHist.Count)
	}

	if queryFreqHist := metrics.GetHistogram("query_frequency"); queryFreqHist != nil {
		fmt.Printf("- 查询频率分布: 均值=%.2f, 最大=%.2f (共%d次)\n",
			queryFreqHist.Mean, queryFreqHist.Max, queryFreqHist.Count)
	}

	// 显示实时指标
	fmt.Printf("\n🔄 实时指标:\n")
	fmt.Printf("- 当前查询负载: %.1f\n", metrics.GetGauge("current_query_load"))
	fmt.Printf("- 平均转账金额: %.2f\n", metrics.GetGauge("avg_transfer_amount"))
	fmt.Printf("==================\n")

	// 模拟用户数量变化
	metrics.SetGauge("online_users", 1200)
	fmt.Printf("- 用户数增长到: %.0f\n", metrics.GetGauge("online_users"))

	// 显示各账户状态
	fmt.Println("\n💰 各账户运行状态:")
	for i := range accounts {
		fmt.Printf("- 账户%03d: 运行正常\n", i+1)
	}

	fmt.Println("\n✨ 演示结束，系统将自然停止。")
}
