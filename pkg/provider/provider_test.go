package provider_test

import (
	"testing"

	"github.com/kercylan98/vivid/pkg/provider"
)

// TestProvider 测试 Provider 接口的基本功能
func TestProvider(t *testing.T) {
	// 测试字符串提供者
	stringProvider := provider.FN[string](func() string {
		return "hello world"
	})

	result := stringProvider.Provide()
	if result != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", result)
	}
}

// TestFNProvider 测试 FN 类型的提供者功能
func TestFNProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider provider.Provider[int]
		expected int
	}{
		{
			name: "simple int provider",
			provider: provider.FN[int](func() int {
				return 42
			}),
			expected: 42,
		},
		{
			name: "zero value provider",
			provider: provider.FN[int](func() int {
				return 0
			}),
			expected: 0,
		},
		{
			name: "negative value provider",
			provider: provider.FN[int](func() int {
				return -100
			}),
			expected: -100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.provider.Provide()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestStructProvider 测试结构体类型的提供者
func TestStructProvider(t *testing.T) {
	type TestStruct struct {
		Name string
		Age  int
	}

	structProvider := provider.FN[TestStruct](func() TestStruct {
		return TestStruct{
			Name: "Alice",
			Age:  30,
		}
	})

	result := structProvider.Provide()
	if result.Name != "Alice" {
		t.Errorf("Expected name 'Alice', got '%s'", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("Expected age 30, got %d", result.Age)
	}
}

// TestPointerProvider 测试指针类型的提供者
func TestPointerProvider(t *testing.T) {
	type TestStruct struct {
		Value string
	}

	pointerProvider := provider.FN[*TestStruct](func() *TestStruct {
		return &TestStruct{Value: "test"}
	})

	result := pointerProvider.Provide()
	if result == nil {
		t.Error("Expected non-nil pointer")
		return
	}
	if result.Value != "test" {
		t.Errorf("Expected value 'test', got '%s'", result.Value)
	}
}

// TestSliceProvider 测试切片类型的提供者
func TestSliceProvider(t *testing.T) {
	sliceProvider := provider.FN[[]int](func() []int {
		return []int{1, 2, 3, 4, 5}
	})

	result := sliceProvider.Provide()
	expected := []int{1, 2, 3, 4, 5}

	if len(result) != len(expected) {
		t.Errorf("Expected length %d, got %d", len(expected), len(result))
		return
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
		}
	}
}

// TestMapProvider 测试映射类型的提供者
func TestMapProvider(t *testing.T) {
	mapProvider := provider.FN[map[string]int](func() map[string]int {
		return map[string]int{
			"one":   1,
			"two":   2,
			"three": 3,
		}
	})

	result := mapProvider.Provide()
	expected := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
	}

	if len(result) != len(expected) {
		t.Errorf("Expected map length %d, got %d", len(expected), len(result))
		return
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("Expected %d for key '%s', got %d", v, k, result[k])
		}
	}
}

// TestMultipleProvides 测试多次调用 Provide 方法
func TestMultipleProvides(t *testing.T) {
	counter := 0
	countingProvider := provider.FN[int](func() int {
		counter++
		return counter
	})

	// 第一次调用
	result1 := countingProvider.Provide()
	if result1 != 1 {
		t.Errorf("Expected 1 on first call, got %d", result1)
	}

	// 第二次调用
	result2 := countingProvider.Provide()
	if result2 != 2 {
		t.Errorf("Expected 2 on second call, got %d", result2)
	}

	// 第三次调用
	result3 := countingProvider.Provide()
	if result3 != 3 {
		t.Errorf("Expected 3 on third call, got %d", result3)
	}
}

// BenchmarkFNProvider 性能测试
func BenchmarkFNProvider(b *testing.B) {
	p := provider.FN[int](func() int {
		return 42
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Provide()
	}
}

// BenchmarkStructProvider 结构体提供者性能测试
func BenchmarkStructProvider(b *testing.B) {
	type TestStruct struct {
		Name string
		Age  int
	}

	p := provider.FN[TestStruct](func() TestStruct {
		return TestStruct{
			Name: "Benchmark",
			Age:  25,
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Provide()
	}
}
