package utils

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// NewExponentialBackoffWithDefault 创建新的指数退避实例，使用默认参数
//
// 参数:
//   - initialDelay: 初始延迟时间
//   - maxDelay: 最大延迟时间
//   - factor: 退避因子，默认为2.0
//   - jitter: 是否启用抖动
//
// 返回:
//   - *ExponentialBackoff: 指数退避实例
func NewExponentialBackoffWithDefault(initialDelay, maxDelay time.Duration) *ExponentialBackoff {
	return NewExponentialBackoff(initialDelay, maxDelay, 2.0, true)
}

// NewExponentialBackoff 创建新的指数退避实例
//
// 参数:
//   - initialDelay: 初始延迟时间
//   - maxDelay: 最大延迟时间
//   - factor: 退避因子
//   - jitter: 是否启用抖动
//
// 返回:
//   - *ExponentialBackoff: 指数退避实例
func NewExponentialBackoff(initialDelay, maxDelay time.Duration, factor float64, jitter bool) *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialDelay:   initialDelay,
		MaxDelay:       maxDelay,
		Factor:         factor,
		Jitter:         jitter,
		currentAttempt: 0,
	}
}

// ExponentialBackoff 指数退避结构体
type ExponentialBackoff struct {
	InitialDelay   time.Duration // InitialDelay 初始延迟时间
	MaxDelay       time.Duration // MaxDelay 最大延迟时间
	Factor         float64       // Factor 退避因子
	Jitter         bool          // Jitter 是否启用抖动，避免雷群效应
	currentAttempt int           // currentAttempt 当前尝试次数
}

// Try 尝试执行函数，如果失败则进行指数退避
//
// 参数:
//   - limit: 最大尝试次数，如果 < 0 则不限制
//   - fn: 要执行的函数
//
// 返回:
//   - abort: 是否终止
//   - err: 执行函数的结果
func (eb *ExponentialBackoff) Try(limit int, fn func() (abort bool, err error)) (abort bool, err error) {
	defer func() {
		eb.Reset()
	}()
	for {
		abort, err = fn()
		if abort || err == nil {
			return abort, err
		}
		if limit >= 0 && eb.currentAttempt >= limit {
			return abort, fmt.Errorf("try failed after %d attempts", eb.currentAttempt)
		}
		time.Sleep(eb.Next())
	}
}

// Forever 无限循环执行函数，直到成功或主动中断。
//
// 参数:
//   - fn: 要执行的函数
//   - errorHandlers: 错误处理函数，用于处理错误，按顺序执行，如果存在多个错误处理函数，则按顺序执行，如果执行函数返回 true，则终止循环
//
// 返回:
//   - abort: 是否终止
//   - err: 执行函数的结果
func (eb *ExponentialBackoff) Forever(fn func() (abort bool, err error), errorHandlers ...func(err error)) (abort bool, err error) {
	defer func() {
		eb.Reset()
	}()
	for {
		abort, err = fn()
		if err != nil {
			for _, errorHandler := range errorHandlers {
				errorHandler(err)
			}
		}
		if abort || err == nil {
			return abort, err
		}
		time.Sleep(eb.Next())
	}
}

// Next 获取下一次的延迟时间
//
// 返回:
//   - time.Duration: 下一次的延迟时间
func (eb *ExponentialBackoff) Next() time.Duration {
	// 计算指数延迟: initialDelay * (factor ^ attempt)
	delay := float64(eb.InitialDelay) * math.Pow(eb.Factor, float64(eb.currentAttempt))

	// 限制在最大延迟范围内
	if delay > float64(eb.MaxDelay) {
		delay = float64(eb.MaxDelay)
	}

	// 应用抖动（如果启用）
	if eb.Jitter {
		// 添加 ±25% 的随机抖动
		jitterAmount := delay * 0.25
		jitter := (rand.Float64()*2 - 1) * jitterAmount // -0.25 到 +0.25
		delay += jitter
		// 确保延迟不为负数
		if delay < 0 {
			delay = 0
		}
	}

	// 增加尝试次数
	eb.currentAttempt++

	return time.Duration(delay)
}

// Reset 重置退避状态，将尝试次数归零
func (eb *ExponentialBackoff) Reset() {
	eb.currentAttempt = 0
}

// GetAttempt 获取当前尝试次数
func (eb *ExponentialBackoff) GetAttempt() int {
	return eb.currentAttempt
}
