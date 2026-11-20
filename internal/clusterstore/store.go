// Package clusterstore 提供协调元数据的确定性状态机和本地日志，不负责共识或线性一致读取。
package clusterstore

import (
	"bytes"
	"errors"
	"unicode/utf8"
)

const (
	maxKeyBytes   = 1024
	maxValueBytes = 1 << 20
	maxEntries    = 1 << 16
	maxStateBytes = 32 << 20 // 键和值的累计字节，不代表实际堆内存上限
)

var (
	ErrInvalidCommand = errors.New("invalid coordination command")
	ErrInvalidIndex   = errors.New("coordination index must advance by one")
	ErrCapacity       = errors.New("coordination state capacity exceeded")
)

type Operation uint8

const (
	Noop Operation = iota // 共识日志中的空操作及配置记录仍推进应用位置
	Put
	Delete
)

// Command 的比较与写入属于同一次状态转换；ExpectedRevision 为 0 时要求键不存在。
// 所有写入均使用 CAS，Value 在 Apply 期间不得修改，成功后由状态机保存副本。
type Command struct {
	Operation        Operation
	Key              string
	Value            []byte
	ExpectedRevision uint64
}

// Record 的 Revision 来自成功写入的日志位置，删除后重建不会复用旧版本。
type Record struct {
	Value    []byte
	Revision uint64
}

// Result 记录单条已提交命令的执行结果；比较失败及容量不足也消耗该日志位置。
type Result struct {
	Applied  bool
	Revision uint64 // 当前记录的版本，键不存在时为 0
	Err      error
}

// Store 由共识应用流程串行使用，不得将本地 Get 当成线性一致读取。
// 零值可用，使用后不可复制；不访问时间、随机数、网络或用户回调，副本按相同日志得到相同结果。
type Store struct {
	records map[string]Record
	index   uint64
	bytes   int
}

// Index 返回已经消费的日志位置，包含未改变记录的命令。
func (s *Store) Index() uint64 { return s.index }

// Get 返回值副本，调用方修改不会影响后续应用或快照。
func (s *Store) Get(key string) (Record, bool) {
	record, exists := s.records[key]
	record.Value = bytes.Clone(record.Value)
	return record, exists
}

// Apply 仅接收顺序提交的日志；索引或命令格式错误不推进位置，应由调用方停止应用并报告。
// 业务比较失败与容量不足通过 Result 返回，仍推进位置，不能回退并重新执行该日志。
func (s *Store) Apply(index uint64, command Command) (Result, error) {
	if index == 0 || s.index == ^uint64(0) || index != s.index+1 {
		return Result{}, ErrInvalidIndex
	}
	if err := validateCommand(command); err != nil {
		return Result{}, err
	}
	s.index = index
	if command.Operation == Noop {
		return Result{Applied: true}, nil
	}
	current, exists := s.records[command.Key]
	result := Result{Revision: current.Revision}
	if current.Revision != command.ExpectedRevision {
		return result, nil
	}
	if command.Operation == Delete {
		if exists {
			s.bytes -= len(command.Key) + len(current.Value)
			delete(s.records, command.Key)
		}
		return Result{Applied: true}, nil
	}
	newBytes := s.bytes + len(command.Value) - len(current.Value)
	if !exists {
		newBytes += len(command.Key)
	}
	if newBytes > maxStateBytes || !exists && len(s.records) >= maxEntries {
		result.Err = ErrCapacity
		return result, nil
	}
	if s.records == nil {
		s.records = make(map[string]Record)
	}
	s.records[command.Key] = Record{Value: bytes.Clone(command.Value), Revision: index}
	s.bytes = newBytes
	return Result{Applied: true, Revision: index}, nil
}

func validateCommand(command Command) error {
	switch command.Operation {
	case Noop:
		if command.Key != "" || len(command.Value) != 0 || command.ExpectedRevision != 0 {
			return ErrInvalidCommand
		}
	case Put, Delete:
		if !validKey(command.Key) || len(command.Value) > maxValueBytes || command.Operation == Delete && len(command.Value) != 0 {
			return ErrInvalidCommand
		}
	default:
		return ErrInvalidCommand
	}
	return nil
}

func validKey(key string) bool {
	return len(key) > 0 && len(key) <= maxKeyBytes && utf8.ValidString(key)
}
