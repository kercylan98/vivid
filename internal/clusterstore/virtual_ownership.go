package clusterstore

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/kercylan98/vivid/internal/utils"
)

const (
	virtualOwnershipPrefix = "virtual:"
	virtualOwnerMagic      = "VIVIDVO1"
	virtualOwnerHeader     = 28
	maxVirtualAddressBytes = 512
)

var (
	ErrInvalidVirtualOwnership  = errors.New("invalid virtual actor ownership")
	ErrVirtualOwnershipConflict = errors.New("virtual actor ownership comparison failed")
)

// VirtualOwner 标识一次所有者启动，地址只用于联系，不能代替启动身份。
// 同一次启动的所有虚拟 Actor 共用此值；同地址重启须重新生成，不随 Raft Leader 切换而更换。
type VirtualOwner struct {
	InstanceID [16]byte
	Address    string
}

func NewVirtualOwner(address string) (VirtualOwner, error) {
	if !validVirtualAddress(address) {
		return VirtualOwner{}, ErrInvalidVirtualOwnership
	}
	owner := VirtualOwner{Address: address}
	for owner.InstanceID == [16]byte{} {
		if _, err := rand.Read(owner.InstanceID[:]); err != nil {
			return VirtualOwner{}, err
		}
	}
	return owner, nil
}

// VirtualOwnership 的 Generation 来自成功取得归属的已提交日志位置。
// 删除重建不复用代次；仅在同一未回退的共识历史内比较，不跨重新创建的组使用。
// 归属不表示业务已经恢复或激活，也不提供租约、物理路由或外部副作用隔离。
type VirtualOwnership struct {
	Kind, Name string
	Owner      VirtualOwner
	Generation uint64
}

// VirtualOwnershipChange 保存一次确定的 CAS 意图，不自动重试，也不缓存执行结果。
// 只有成功回执可产生本次代次，不保证消费回执时仍持有归属。
// 结果未知时不能根据本地提案位置生成代次。
type VirtualOwnershipChange struct {
	key       string
	ownership VirtualOwnership
	expected  uint64
	release   bool
}

// VirtualOwnershipKey 与虚拟引用使用相同的转义身份，避免分隔符混淆 kind 与 name。
func VirtualOwnershipKey(kind, name string) (string, error) {
	if kind == "" || name == "" || len(kind) > maxKeyBytes || len(name) > maxKeyBytes || !utf8.ValidString(kind) || !utf8.ValidString(name) {
		return "", ErrInvalidVirtualOwnership
	}
	key := virtualOwnershipPrefix + url.PathEscape(kind) + "/" + url.PathEscape(name)
	if !validKey(key) {
		return "", ErrInvalidVirtualOwnership
	}
	return key, nil
}

// ClaimVirtualOwnership 比较归属代次并准备取得记录；0 要求不存在。
// 非零比较用于显式重新分配，调用方仍须决定接管策略并隔离旧所有者，不能由 Gossip 不可达直接触发。
func ClaimVirtualOwnership(kind, name string, owner VirtualOwner, expectedGeneration uint64) (VirtualOwnershipChange, error) {
	key, err := VirtualOwnershipKey(kind, name)
	if err != nil || !validVirtualOwner(owner) {
		return VirtualOwnershipChange{}, ErrInvalidVirtualOwnership
	}
	return VirtualOwnershipChange{key: key, ownership: VirtualOwnership{Kind: kind, Name: name, Owner: owner}, expected: expectedGeneration}, nil
}

// Release 只删除所持代次，不会释放同地址重启或重新取得后的新归属。
// 完成业务收尾后再释放；删除记录本身不会停止旧 Actor。
func (o VirtualOwnership) Release() (VirtualOwnershipChange, error) {
	key, err := VirtualOwnershipKey(o.Kind, o.Name)
	if err != nil || o.Generation == 0 || !validVirtualOwner(o.Owner) {
		return VirtualOwnershipChange{}, ErrInvalidVirtualOwnership
	}
	return VirtualOwnershipChange{key: key, ownership: o, expected: o.Generation, release: true}, nil
}

// Request 每次构造独立负载，可交给同组 RaftActor 的执行入口；保持原有等待和错误语义。
func (c VirtualOwnershipChange) Request(timeout time.Duration) RaftRuntimeExecute {
	command := Command{Operation: Delete, Key: c.key, ExpectedRevision: c.expected}
	if !c.release {
		command.Operation, command.Value = Put, encodeVirtualOwner(c.ownership.Owner)
	}
	return RaftRuntimeExecute{Command: command, Timeout: timeout}
}

// Result 只消费与本次 Request 对应的实际执行回执，冲突、容量拒绝和结果未知均不返回归属。
// 释放成功返回零值，取得成功使用执行位置作为新代次，不延续被比较的旧代次。
func (c VirtualOwnershipChange) Result(result RaftRuntimeExecution) (VirtualOwnership, error) {
	if c.key == "" {
		return VirtualOwnership{}, ErrInvalidVirtualOwnership
	}
	if result.Err != nil {
		return VirtualOwnership{}, result.Err
	}
	if result.Result.Err != nil {
		return VirtualOwnership{}, result.Result.Err
	}
	if !result.Result.Applied {
		return VirtualOwnership{}, ErrVirtualOwnershipConflict
	}
	if result.Term == 0 || result.Index == 0 || result.Index <= c.expected {
		return VirtualOwnership{}, ErrInvalidVirtualOwnership
	}
	if c.release {
		if result.Result.Revision != 0 {
			return VirtualOwnership{}, ErrInvalidVirtualOwnership
		}
		return VirtualOwnership{}, nil
	}
	if result.Result.Revision != result.Index {
		return VirtualOwnership{}, ErrInvalidVirtualOwnership
	}
	ownership := c.ownership
	ownership.Generation = result.Index
	return ownership, nil
}

// ReadVirtualOwnership 解析同组 RaftRuntimeGet 的对应回执，拒绝本地诊断读取和带错误的数据。
// 成功只表示本次读取的线性一致结果，不能据此直接激活或授权后续写入。
func ReadVirtualOwnership(kind, name string, result RaftRuntimeRecord) (VirtualOwnership, bool, error) {
	if result.Err != nil {
		return VirtualOwnership{}, false, result.Err
	}
	if _, err := VirtualOwnershipKey(kind, name); err != nil {
		return VirtualOwnership{}, false, err
	}
	if result.Term == 0 || result.Index == 0 {
		return VirtualOwnership{}, false, ErrInvalidVirtualOwnership
	}
	if !result.Exists {
		if result.Revision != 0 || len(result.Value) != 0 {
			return VirtualOwnership{}, false, ErrInvalidVirtualOwnership
		}
		return VirtualOwnership{}, false, nil
	}
	if result.Revision == 0 || result.Revision > result.Index {
		return VirtualOwnership{}, false, ErrInvalidVirtualOwnership
	}
	owner, err := decodeVirtualOwner(result.Value)
	if err != nil {
		return VirtualOwnership{}, false, err
	}
	return VirtualOwnership{Kind: kind, Name: name, Owner: owner, Generation: result.Revision}, true, nil
}

func encodeVirtualOwner(owner VirtualOwner) []byte {
	data := make([]byte, virtualOwnerHeader+len(owner.Address))
	copy(data, virtualOwnerMagic)
	copy(data[8:24], owner.InstanceID[:])
	binary.LittleEndian.PutUint32(data[24:], uint32(len(owner.Address)))
	copy(data[virtualOwnerHeader:], owner.Address)
	return data
}

func decodeVirtualOwner(data []byte) (VirtualOwner, error) {
	if len(data) < virtualOwnerHeader || len(data) > virtualOwnerHeader+maxVirtualAddressBytes || string(data[:8]) != virtualOwnerMagic {
		return VirtualOwner{}, ErrInvalidVirtualOwnership
	}
	if uint64(binary.LittleEndian.Uint32(data[24:])) != uint64(len(data)-virtualOwnerHeader) {
		return VirtualOwner{}, ErrInvalidVirtualOwnership
	}
	owner := VirtualOwner{Address: string(data[virtualOwnerHeader:])}
	copy(owner.InstanceID[:], data[8:24])
	if !validVirtualOwner(owner) {
		return VirtualOwner{}, ErrInvalidVirtualOwnership
	}
	return owner, nil
}

func validVirtualOwner(owner VirtualOwner) bool {
	return owner.InstanceID != [16]byte{} && validVirtualAddress(owner.Address)
}

func validVirtualAddress(address string) bool {
	if len(address) > maxVirtualAddressBytes || !utf8.ValidString(address) {
		return false
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || !utils.IsValidHost(host) {
		return false
	}
	number, err := strconv.ParseUint(port, 10, 16)
	return err == nil && number != 0
}
