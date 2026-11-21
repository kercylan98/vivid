package clusterstore

import (
	"encoding/binary"

	"github.com/kercylan98/vivid/internal/serialization"
)

const (
	raftPacketMagic       = "VIVIDRM1"
	raftPacketHeaderBytes = 124
	maxRaftPacketBytes    = (4 << 20) - (64 << 10) // 为 Remoting 信封及两个有界引用预留空间
	maxRaftRouteBytes     = 4 << 10
)

// RaftWire 只承载有界协议字节，完整快照必须经分块协议发送。
// 解码复制负载，不能在网络 Reader 归还后继续借用它的输入。
type RaftWire struct{ Data []byte }

func (*RaftWire) Encode(w *serialization.Writer, message any) error {
	data := message.(*RaftWire).Data
	if len(data) == 0 || len(data) > maxRaftPacketBytes {
		return ErrInvalidRaftMessage
	}
	w.WriteBytes(data)
	return w.Err()
}

func (*RaftWire) Decode(r *serialization.Reader, message any) error {
	wire := message.(*RaftWire)
	if err := r.ReadBytes(&wire.Data); err != nil {
		return err
	}
	if len(wire.Data) == 0 || len(wire.Data) > maxRaftPacketBytes {
		return ErrInvalidRaftMessage
	}
	return nil
}

func raftPacketSize(m RaftMessage) (int, error) {
	if m.Type < RaftVoteRequest || m.Type > RaftAppendResponse || !validRaftMessage(m) {
		return 0, ErrInvalidRaftMessage
	}
	size := raftPacketHeaderBytes
	for _, entry := range m.Entries {
		size += raftEntryHeaderBytes + len(entry.Data)
		if size > maxRaftPacketBytes {
			return 0, ErrInvalidRaftMessage
		}
	}
	return size, nil
}

// EncodeRaftPacket 编码投票或普通复制消息，CRC32C 覆盖头部与负载，不提供认证。
func EncodeRaftPacket(m RaftMessage) ([]byte, error) {
	size, err := raftPacketSize(m)
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	copy(data, raftPacketMagic)
	data[8] = byte(m.Type)
	if m.Reject {
		data[9] = 1
	}
	copy(data[16:48], m.Cluster[:])
	values := [...]uint64{m.From, m.To, m.Term, m.Request, m.Index, m.LogTerm, m.Commit, m.Match, m.Hint}
	for i, value := range values {
		binary.LittleEndian.PutUint64(data[48+i*8:], value)
	}
	binary.LittleEndian.PutUint32(data[120:], uint32(len(m.Entries)))
	offset := raftPacketHeaderBytes
	for _, entry := range m.Entries {
		binary.LittleEndian.PutUint64(data[offset:], entry.Index)
		binary.LittleEndian.PutUint64(data[offset+8:], entry.Term)
		binary.LittleEndian.PutUint32(data[offset+16:], uint32(len(entry.Data)))
		copy(data[offset+raftEntryHeaderBytes:], entry.Data)
		offset += raftEntryHeaderBytes + len(entry.Data)
	}
	binary.LittleEndian.PutUint32(data[12:16], packetChecksum(data))
	return data, nil
}

// DecodeRaftPacket 先限制长度和验证完整性；条目负载借用输入，直至本次 Step 返回。
func DecodeRaftPacket(data []byte) (RaftMessage, error) {
	if len(data) < raftPacketHeaderBytes || len(data) > maxRaftPacketBytes || string(data[:8]) != raftPacketMagic || data[9] > 1 || data[10] != 0 || data[11] != 0 || binary.LittleEndian.Uint32(data[12:16]) != packetChecksum(data) {
		return RaftMessage{}, ErrInvalidRaftMessage
	}
	m := RaftMessage{Type: RaftMessageType(data[8]), Reject: data[9] != 0}
	copy(m.Cluster[:], data[16:48])
	values := [...]*uint64{&m.From, &m.To, &m.Term, &m.Request, &m.Index, &m.LogTerm, &m.Commit, &m.Match, &m.Hint}
	for i, value := range values {
		*value = binary.LittleEndian.Uint64(data[48+i*8:])
	}
	count := binary.LittleEndian.Uint32(data[120:])
	data = data[raftPacketHeaderBytes:]
	if count > maxRaftMessageEntries || uint64(count)*raftEntryHeaderBytes > uint64(len(data)) {
		return RaftMessage{}, ErrInvalidRaftMessage
	}
	if count != 0 {
		m.Entries = make([]RaftEntry, 0, count)
	}
	for range count {
		if len(data) < raftEntryHeaderBytes {
			return RaftMessage{}, ErrInvalidRaftMessage
		}
		entry := RaftEntry{Index: binary.LittleEndian.Uint64(data), Term: binary.LittleEndian.Uint64(data[8:])}
		size := uint64(binary.LittleEndian.Uint32(data[16:]))
		data = data[raftEntryHeaderBytes:]
		if size > maxRaftEntryBytes || size > uint64(len(data)) {
			return RaftMessage{}, ErrInvalidRaftMessage
		}
		entry.Data = data[:size:size]
		m.Entries = append(m.Entries, entry)
		data = data[size:]
	}
	if len(data) != 0 {
		return RaftMessage{}, ErrInvalidRaftMessage
	}
	if _, err := raftPacketSize(m); err != nil {
		return RaftMessage{}, err
	}
	return m, nil
}
