package clusterstore

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"sync"
	"sync/atomic"
	"time"
)

const (
	SnapshotChunkBytes        = 256 << 10
	snapshotPacketHeaderBytes = 168
	snapshotPacketMagic       = "VIVIDST1"
)

var (
	snapshotPacketCRCTable      = crc32.MakeTable(crc32.Castagnoli)
	ErrInvalidSnapshotPacket    = errors.New("invalid snapshot transfer packet")
	ErrSnapshotTransferCapacity = errors.New("snapshot transfer capacity exceeded")
	ErrSnapshotTransferBusy     = errors.New("snapshot transfer frame is still in use")
	ErrSnapshotTransferClosed   = errors.New("snapshot transfer is closed")
	ErrSnapshotTransferStale    = errors.New("snapshot transfer session is stale")
)

type SnapshotPacketType uint8

const (
	SnapshotChunk SnapshotPacketType = iota + 1
	SnapshotChunkAck
)

// SnapshotTransferID 绑定完整会话，不以地址或可复用的请求编号独自判断归属。
type SnapshotTransferID struct {
	Cluster                 [sha256.Size]byte
	Session                 [16]byte
	Digest                  [sha256.Size]byte
	From, To, Term, Request uint64
	Index, LogTerm, Size    uint64
}

// SnapshotPacket 的应答保留原发送方与接收方，Offset 表示接收端已连续取得的字节数。
// Done 只用于完整快照已经持久化的应答，普通块确认不推进 Raft 的复制位置。
type SnapshotPacket struct {
	ID           SnapshotTransferID
	Type         SnapshotPacketType
	Done         bool
	Reject       bool
	Offset       uint64
	ReceiverTerm uint64
	Data         []byte
}

func (p SnapshotPacket) valid() bool {
	if p.ID.Session == [16]byte{} || p.ID.From == 0 || p.ID.To == 0 || p.ID.From == p.ID.To || p.ID.Term == 0 || p.ID.Request == 0 || p.ID.Index == 0 || p.ID.LogTerm == 0 || p.ID.LogTerm > p.ID.Term || p.ID.Size == 0 || p.ID.Size > maxSnapshotBytes || p.Offset > p.ID.Size {
		return false
	}
	switch p.Type {
	case SnapshotChunk:
		return !p.Done && !p.Reject && p.ReceiverTerm == 0 && p.Offset < p.ID.Size && p.Offset%SnapshotChunkBytes == 0 && uint64(len(p.Data)) == min(uint64(SnapshotChunkBytes), p.ID.Size-p.Offset)
	case SnapshotChunkAck:
		return len(p.Data) == 0 && p.ReceiverTerm >= p.ID.Term && (!p.Reject || !p.Done && p.Offset == 0) && (p.Offset == p.ID.Size && p.Done || p.Offset < p.ID.Size && !p.Done && p.Offset%SnapshotChunkBytes == 0)
	}
	return false
}

// EncodeSnapshotPacket 生成独立的有界字节串，CRC32C 覆盖包头与负载，不提供认证。
func EncodeSnapshotPacket(p SnapshotPacket) ([]byte, error) {
	if !p.valid() {
		return nil, ErrInvalidSnapshotPacket
	}
	data := make([]byte, snapshotPacketHeaderBytes+len(p.Data))
	copy(data, snapshotPacketMagic)
	data[8] = byte(p.Type)
	if p.Done {
		data[9] |= 1
	}
	if p.Reject {
		data[9] |= 2
	}
	copy(data[16:48], p.ID.Cluster[:])
	copy(data[48:64], p.ID.Session[:])
	copy(data[64:96], p.ID.Digest[:])
	values := [...]uint64{p.ID.From, p.ID.To, p.ID.Term, p.ID.Request, p.ID.Index, p.ID.LogTerm, p.ID.Size, p.Offset, p.ReceiverTerm}
	for i, value := range values {
		binary.LittleEndian.PutUint64(data[96+i*8:], value)
	}
	copy(data[snapshotPacketHeaderBytes:], p.Data)
	binary.LittleEndian.PutUint32(data[12:16], packetChecksum(data))
	return data, nil
}

func packetChecksum(data []byte) uint32 {
	checksum := crc32.Checksum(data[:12], snapshotPacketCRCTable)
	return crc32.Update(checksum, snapshotPacketCRCTable, data[16:])
}

// DecodeSnapshotPacket 借用输入负载，输入须保持有效且不可修改，直至本次 Step 返回。
// 在解析或分配前先限制整包长度；持有解码结果不能提前释放其 SnapshotFrame。
func DecodeSnapshotPacket(data []byte) (SnapshotPacket, error) {
	if len(data) < snapshotPacketHeaderBytes || len(data) > snapshotPacketHeaderBytes+SnapshotChunkBytes || string(data[:8]) != snapshotPacketMagic || data[9]&^byte(3) != 0 || data[10] != 0 || data[11] != 0 || binary.LittleEndian.Uint32(data[12:16]) != packetChecksum(data) {
		return SnapshotPacket{}, ErrInvalidSnapshotPacket
	}
	p := SnapshotPacket{Type: SnapshotPacketType(data[8]), Done: data[9]&1 != 0, Reject: data[9]&2 != 0}
	copy(p.ID.Cluster[:], data[16:48])
	copy(p.ID.Session[:], data[48:64])
	copy(p.ID.Digest[:], data[64:96])
	values := [...]*uint64{&p.ID.From, &p.ID.To, &p.ID.Term, &p.ID.Request, &p.ID.Index, &p.ID.LogTerm, &p.ID.Size, &p.Offset, &p.ReceiverTerm}
	for i, value := range values {
		*value = binary.LittleEndian.Uint64(data[96+i*8:])
	}
	p.Data = data[snapshotPacketHeaderBytes:]
	if !p.valid() {
		return SnapshotPacket{}, ErrInvalidSnapshotPacket
	}
	return p, nil
}

type SnapshotTransferLimits struct {
	MaxSessions      int
	MaxSnapshotBytes uint64 // 收发完整缓冲的累计预留量，不包含解析和业务状态机分配
	MaxInFlightBytes uint64 // 编码发送缓冲，写出结束后才归还
	Timeout          time.Duration
}

type SnapshotTransferUsage struct {
	Sessions                     int
	SnapshotBytes, InFlightBytes uint64
}

// SnapshotTransferBudget 可在同一进程的多个传输器间共享，释放发送帧允许跨线程调用。
// 额度约束本组件拥有的缓冲，不能约束调用方复制、传输队列或解码状态机的总堆内存。
type SnapshotTransferBudget struct {
	mu     sync.Mutex
	limits SnapshotTransferLimits
	usage  SnapshotTransferUsage
}

func NewSnapshotTransferBudget(limits SnapshotTransferLimits) (*SnapshotTransferBudget, error) {
	if limits.MaxSessions == 0 {
		limits.MaxSessions = 4
	}
	if limits.MaxSnapshotBytes == 0 {
		limits.MaxSnapshotBytes = 128 << 20
	}
	if limits.MaxInFlightBytes == 0 {
		limits.MaxInFlightBytes = 4 * (SnapshotChunkBytes + snapshotPacketHeaderBytes)
	}
	if limits.Timeout == 0 {
		limits.Timeout = 30 * time.Second
	}
	if limits.MaxSessions < 1 || limits.MaxSnapshotBytes < 1 || limits.MaxInFlightBytes < SnapshotChunkBytes+snapshotPacketHeaderBytes || limits.Timeout < 0 {
		return nil, ErrSnapshotTransferCapacity
	}
	return &SnapshotTransferBudget{limits: limits}, nil
}

func (b *SnapshotTransferBudget) Usage() SnapshotTransferUsage {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.usage
}

func (b *SnapshotTransferBudget) reserve(size uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.usage.Sessions >= b.limits.MaxSessions || size > b.limits.MaxSnapshotBytes-b.usage.SnapshotBytes {
		return ErrSnapshotTransferCapacity
	}
	b.usage.Sessions++
	b.usage.SnapshotBytes += size
	return nil
}

func (b *SnapshotTransferBudget) release(size uint64) {
	b.mu.Lock()
	b.usage.Sessions--
	b.usage.SnapshotBytes -= size
	b.mu.Unlock()
}

// SnapshotFrame 在实际写出完成或最终放弃后 Release；会话取消不能提前释放仍在写出的缓冲。
// Release 允许重复和并发调用；调用前不得再读取 Bytes 或持有它的解码负载。
type SnapshotFrame struct {
	Bytes    []byte
	budget   *SnapshotTransferBudget
	size     uint64
	released atomic.Bool
}

func (f *SnapshotFrame) Release() {
	if f == nil || !f.released.CompareAndSwap(false, true) {
		return
	}
	f.Bytes = nil
	f.budget.mu.Lock()
	f.budget.usage.InFlightBytes -= f.size
	f.budget.mu.Unlock()
}

func (b *SnapshotTransferBudget) frame(p SnapshotPacket) (*SnapshotFrame, error) {
	size := uint64(snapshotPacketHeaderBytes + len(p.Data))
	b.mu.Lock()
	if size > b.limits.MaxInFlightBytes-b.usage.InFlightBytes {
		b.mu.Unlock()
		return nil, ErrSnapshotTransferCapacity
	}
	b.usage.InFlightBytes += size
	b.mu.Unlock()
	data, err := EncodeSnapshotPacket(p)
	if err != nil {
		b.mu.Lock()
		b.usage.InFlightBytes -= size
		b.mu.Unlock()
		return nil, err
	}
	return &SnapshotFrame{Bytes: data, budget: b, size: size}, nil
}
