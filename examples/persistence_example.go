package main

import (
	"fmt"
	"log"
	"time"

	"github.com/kercylan98/vivid/src/persistence/persistencerepos"
	"github.com/kercylan98/vivid/src/vivid"
)

// CounterState 计数器状态
type CounterState struct {
	Count int `json:"count"`
}

// IncrementEvent 增加事件
type IncrementEvent struct {
	Delta int `json:"delta"`
}

// CounterActor 是一个支持持久化的计数器 Actor
type CounterActor struct {
	state *CounterState
}

// GetPersistenceId 返回持久化 ID
func (c *CounterActor) GetPersistenceId() string {
	return "counter-actor-1"
}

// OnRecover 从持久化存储中恢复状态
func (c *CounterActor) OnRecover(ctx vivid.PersistenceContext) {
	// 恢复快照
	if snapshot := ctx.GetSnapshot(); snapshot != nil {
		if state, ok := snapshot.(*CounterState); ok {
			c.state = state
			fmt.Printf("Actor 已从快照恢复: count = %d\n", c.state.Count)
		}
	} else {
		// 如果没有快照，初始化为默认状态
		c.state = &CounterState{Count: 0}
		fmt.Println("Actor 初始化为默认状态")
	}

	// 重放事件
	events := ctx.GetEvents()
	for _, event := range events {
		if incrementEvent, ok := event.(*IncrementEvent); ok {
			c.state.Count += incrementEvent.Delta
			fmt.Printf("重放事件: +%d, 当前 count = %d\n", incrementEvent.Delta, c.state.Count)
		}
	}
}

// OnReceive 处理消息
func (c *CounterActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *IncrementEvent:
		// 持久化事件
		if persistenceCtx := ctx.Persistence(); persistenceCtx != nil {
			persistenceCtx.Persist(msg)
		}

		// 更新状态
		c.state.Count += msg.Delta
		fmt.Printf("收到增加事件: +%d, 当前 count = %d\n", msg.Delta, c.state.Count)

		// 每 10 次增量就创建一个快照
		if c.state.Count%10 == 0 {
			if persistenceCtx := ctx.Persistence(); persistenceCtx != nil {
				persistenceCtx.SaveSnapshot(c.state)
				fmt.Printf("保存快照: count = %d\n", c.state.Count)
			}
		}

	case string:
		if msg == "get" {
			ctx.Reply(c.state.Count)
		} else if msg == "status" {
			fmt.Printf("当前状态: count = %d\n", c.state.Count)
		}
	}
}

func main() {
	// 创建内存持久化仓库
	repository := persistencerepos.NewMemory()

	// 创建 Actor 系统
	system := vivid.NewActorSystem()

	// 启动系统
	system.StartP()

	// 创建持久化 Actor
	counterRef := system.ActorOf(func() vivid.Actor {
		return &CounterActor{}
	}, func(config *vivid.ActorConfig) {
		config.WithName("counter").WithPersistence(repository)
	})

	// 发送一些增量事件
	system.Tell(counterRef, &IncrementEvent{Delta: 3})
	system.Tell(counterRef, &IncrementEvent{Delta: 2})
	system.Tell(counterRef, &IncrementEvent{Delta: 5}) // 这里会触发快照保存
	system.Tell(counterRef, &IncrementEvent{Delta: 1})
	system.Tell(counterRef, &IncrementEvent{Delta: 4})

	// 查看状态
	time.Sleep(100 * time.Millisecond)
	system.Tell(counterRef, "status")

	// 创建另一个相同 ID 的 Actor，它应该从持久化存储中恢复
	fmt.Println("\n创建新的 Actor 实例...")
	counterRef2 := system.ActorOf(func() vivid.Actor {
		return &CounterActor{}
	}, func(config *vivid.ActorConfig) {
		config.WithName("counter2").WithPersistence(repository)
	})

	time.Sleep(100 * time.Millisecond)
	system.Tell(counterRef2, "status")

	// 继续向新 Actor 发送事件
	system.Tell(counterRef2, &IncrementEvent{Delta: 5}) // 这里应该会触发新快照保存 (count=20)

	time.Sleep(100 * time.Millisecond)
	system.Tell(counterRef2, "status")

	// 让程序运行一段时间
	time.Sleep(1 * time.Second)

	// 关闭系统
	system.StopP()
	log.Println("示例程序完成")
}
