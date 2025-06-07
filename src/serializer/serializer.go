// Package serializer 提供了统一的序列化和反序列化接口定义。
// 该包定义了多种序列化器接口，支持不同的使用场景：
// - 基础序列化器：适用于通用的序列化需求
// - 命名序列化器：支持类型名称标识的序列化
// - 泛型序列化器：提供类型安全的序列化操作
package serializer

// Serializer 定义了基础的序列化和反序列化接口。
// 该接口适用于通用的序列化场景，支持任意类型的数据转换。
type Serializer interface {
	// Serialize 将给定的数据序列化为字节数组。
	// 参数 data 为待序列化的数据，可以是任意类型。
	// 返回序列化后的字节数组和可能的错误。
	Serialize(data any) (serializedData []byte, err error)

	// Deserialize 将字节数组反序列化为指定类型的数据。
	// 参数 serializedData 为待反序列化的字节数组。
	// 参数 target 为反序列化的目标对象指针。
	// 返回可能的错误。
	Deserialize(serializedData []byte, target any) (err error)
}

// NameSerializer 定义了带类型名称标识的序列化和反序列化接口。
// 该接口在序列化时会同时返回类型名称，便于类型识别和路由。
type NameSerializer interface {
	// Serialize 将给定的数据序列化为类型名称和字节数组。
	// 参数 data 为待序列化的数据，可以是任意类型。
	// 返回类型名称、序列化后的字节数组和可能的错误。
	Serialize(data any) (typeName string, serializedData []byte, err error)

	// Deserialize 根据类型名称将字节数组反序列化为指定类型的数据。
	// 参数 typeName 为数据的类型名称。
	// 参数 serializedData 为待反序列化的字节数组。
	// 返回反序列化对象及可能的错误。
	Deserialize(typeName string, serializedData []byte) (result any, err error)
}

// GenericSerializer 定义了泛型的序列化和反序列化接口。
// 该接口提供了类型安全的序列化操作，避免了类型断言的需要。
type GenericSerializer[T any] interface {
	// Serialize 将给定的泛型数据序列化为字节数组。
	// 参数 data 为待序列化的泛型数据。
	// 返回序列化后的字节数组和可能的错误。
	Serialize(data T) (serializedData []byte, err error)

	// Deserialize 将字节数组反序列化为泛型类型的数据。
	// 参数 serializedData 为待反序列化的字节数组。
	// 参数 target 为反序列化的目标对象。
	// 返回可能的错误。
	Deserialize(serializedData []byte, target T) (err error)
}

// GenericNameSerializer 定义了带类型名称标识的泛型序列化和反序列化接口。
// 该接口结合了泛型类型安全和类型名称标识的优势。
type GenericNameSerializer[T any] interface {
	// Serialize 将给定的泛型数据序列化为类型名称和字节数组。
	// 参数 data 为待序列化的泛型数据。
	// 返回类型名称、序列化后的字节数组和可能的错误。
	Serialize(data T) (typeName string, serializedData []byte, err error)

	// Deserialize 根据类型名称将字节数组反序列化为泛型类型的数据。
	// 参数 typeName 为数据的类型名称。
	// 参数 serializedData 为待反序列化的字节数组。
	// 参数 target 为反序列化的目标对象。
	// 返回可能的错误。
	Deserialize(typeName string, serializedData []byte, target T) (err error)
}

type Marshaler interface {
	Marshal() ([]byte, error)
}

type Unmarshaler interface {
	Unmarshal(data []byte) error
}

type MarshalerUnmarshaler interface {
	Marshaler
	Unmarshaler
}
