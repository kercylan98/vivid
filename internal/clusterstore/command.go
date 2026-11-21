package clusterstore

import "encoding/binary"

const (
	commandMagic       = "VIVIDCC1"
	commandHeaderBytes = 32
)

// EncodeCommand 返回独立的有界日志负载，空操作使用 Raft 保留的空负载。
// 完整性由 WAL 和传输层校验，命令格式不提供认证。
func EncodeCommand(command Command) ([]byte, error) {
	if err := validateCommand(command); err != nil {
		return nil, err
	}
	if command.Operation == Noop {
		return nil, nil
	}
	data := make([]byte, commandHeaderBytes+len(command.Key)+len(command.Value))
	copy(data, commandMagic)
	data[8] = byte(command.Operation)
	binary.LittleEndian.PutUint64(data[16:], command.ExpectedRevision)
	binary.LittleEndian.PutUint32(data[24:], uint32(len(command.Key)))
	binary.LittleEndian.PutUint32(data[28:], uint32(len(command.Value)))
	copy(data[commandHeaderBytes:], command.Key)
	copy(data[commandHeaderBytes+len(command.Key):], command.Value)
	return data, nil
}

// DecodeCommand 先校验长度再解析，Value 借用日志负载，直至本次 Apply 返回。
func DecodeCommand(data []byte) (Command, error) {
	if len(data) == 0 {
		return Command{}, nil
	}
	if len(data) < commandHeaderBytes || len(data) > commandHeaderBytes+maxKeyBytes+maxValueBytes || string(data[:8]) != commandMagic {
		return Command{}, ErrInvalidCommand
	}
	for _, value := range data[9:16] {
		if value != 0 {
			return Command{}, ErrInvalidCommand
		}
	}
	keyBytes := uint64(binary.LittleEndian.Uint32(data[24:]))
	valueBytes := uint64(binary.LittleEndian.Uint32(data[28:]))
	if keyBytes == 0 || keyBytes > maxKeyBytes || valueBytes > maxValueBytes || keyBytes+valueBytes != uint64(len(data)-commandHeaderBytes) {
		return Command{}, ErrInvalidCommand
	}
	command := Command{
		Operation: Operation(data[8]), ExpectedRevision: binary.LittleEndian.Uint64(data[16:]),
		Key: string(data[commandHeaderBytes : commandHeaderBytes+keyBytes]), Value: data[commandHeaderBytes+keyBytes:],
	}
	if err := validateCommand(command); err != nil {
		return Command{}, err
	}
	return command, nil
}
