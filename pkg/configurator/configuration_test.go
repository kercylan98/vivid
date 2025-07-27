// Package configurator_test 提供了 configurator 包的测试用例。
package configurator_test

import (
	"github.com/kercylan98/vivid/pkg/configurator"
	"testing"
)

type (
	TestConfigurator   = configurator.Configurator[*TestConfiguration]
	TestConfiguratorFN = configurator.FN[*TestConfiguration]
	TestOption         = configurator.Option[*TestConfiguration]
)

// TestConfiguration 是用于测试的配置结构体。
type TestConfiguration struct {
	Value int
}

// WithValue 设置配置的值并返回自身。
func (c *TestConfiguration) WithValue(value int) *TestConfiguration {
	c.Value = value
	return c
}

// WithTestValue 创建一个设置测试值的选项函数。
func WithTestValue(value int) TestOption {
	return func(configuration *TestConfiguration) {
		configuration.WithValue(value)
	}
}

// TestConfiguratorImpl 是配置器接口的结构体实现。
type TestConfiguratorImpl struct{}

// Configure 实现 Configurator 接口。
func (c *TestConfiguratorImpl) Configure(configuration *TestConfiguration) {
	configuration.WithValue(1)
}

// TestAll 测试配置器的各种使用方式。
func TestAll(t *testing.T) {
	c := new(TestConfiguration)
	
	t.Run("struct configurator", func(t *testing.T) {
		new(TestConfiguratorImpl).Configure(c)
		if c.Value != 1 {
			t.Errorf("struct configurator failed: expected 1, got %d", c.Value)
		}
	})

	t.Run("function configurator", func(t *testing.T) {
		impl := TestConfiguratorFN(func(c *TestConfiguration) {
			c.Value = 2
		})
		impl.Configure(c)
		if c.Value != 2 {
			t.Errorf("function configurator failed: expected 2, got %d", c.Value)
		}
	})

	t.Run("option configurator", func(t *testing.T) {
		option := TestOption(func(configuration *TestConfiguration) {
			configuration.Value = 3
		})
		option.Apply(c)
		if c.Value != 3 {
			t.Errorf("option configurator failed: expected 3, got %d", c.Value)
		}
	})

	t.Run("option with configurator", func(t *testing.T) {
		option := TestOption(func(configuration *TestConfiguration) {
			configuration.Value = 5
		}).With(func(configuration *TestConfiguration) {
			configuration.Value = 4
		})
		option.Apply(c)
		if c.Value != 5 {
			t.Errorf("option with configurator failed: expected 5, got %d", c.Value)
		}
	})
}
