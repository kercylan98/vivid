package processor

import (
    "encoding/binary"
    "errors"
    "github.com/kercylan98/vivid/src/serializer"
)

var (
    _ RPCHandshake = (*rpcHandshake)(nil)
)

func NewRPCHandshake() RPCHandshake {
    return &rpcHandshake{}
}

func NewRPCHandshakeFromBytes(data []byte) (RPCHandshake, error) {
    m := &rpcHandshake{}
    err := m.Unmarshal(data)
    return m, err
}

func NewRPCHandshakeWithAddress(address string) RPCHandshake {
    return &rpcHandshake{
        address: address,
    }
}

type RPCHandshake interface {
    serializer.MarshalerUnmarshaler

    GetAddress() string
}

type rpcHandshake struct {
    address string
}

func (m *rpcHandshake) Marshal() ([]byte, error) {
    addressBytes := []byte(m.address)
    addressLen := len(addressBytes)

    buf := make([]byte, 2+addressLen)
    binary.BigEndian.PutUint16(buf[0:2], uint16(addressLen))
    copy(buf[2:], addressBytes)

    return buf, nil
}

func (m *rpcHandshake) Unmarshal(data []byte) error {
    if len(data) < 2 {
        return errors.New("invalid data length")
    }

    addressLen := binary.BigEndian.Uint16(data[0:2])
    if len(data) < int(2+addressLen) {
        return errors.New("invalid address data")
    }

    m.address = string(data[2 : 2+addressLen])
    return nil
}

func (m *rpcHandshake) GetAddress() string {
    return m.address
}

func NewRPCBatchMessage() RPCBatchMessage {
    return &rpcBatchMessage{}
}

type RPCBatchMessage interface {
    serializer.MarshalerUnmarshaler

    Add(address, path, name string, message []byte)

    Len() int

    Get(index int) (address, path, name string, message []byte)
}

type rpcBatchMessage struct {
    names     []string
    messages  [][]byte
    addresses []string
    paths     []string
}

func (r *rpcBatchMessage) Add(address, path, name string, message []byte) {
    r.addresses = append(r.addresses, address)
    r.paths = append(r.paths, path)
    r.names = append(r.names, name)
    r.messages = append(r.messages, message)
}

func (r *rpcBatchMessage) Len() int {
    return len(r.names)
}

func (r *rpcBatchMessage) Get(index int) (address, path, name string, message []byte) {
    address = r.addresses[index]
    path = r.paths[index]
    name = r.names[index]
    message = r.messages[index]
    return
}
func (r *rpcBatchMessage) Marshal() ([]byte, error) {
    // 计算总长度
    buf := make([]byte, 0)

    // 序列化names
    namesLen := len(r.names)
    namesLenBuf := make([]byte, 2)
    binary.BigEndian.PutUint16(namesLenBuf, uint16(namesLen))
    buf = append(buf, namesLenBuf...)

    for _, name := range r.names {
        nameBytes := []byte(name)
        nameLen := len(nameBytes)
        lenBuf := make([]byte, 2)
        binary.BigEndian.PutUint16(lenBuf, uint16(nameLen))
        buf = append(buf, lenBuf...)
        buf = append(buf, nameBytes...)
    }

    // 序列化messages
    messagesLen := len(r.messages)
    messagesLenBuf := make([]byte, 2)
    binary.BigEndian.PutUint16(messagesLenBuf, uint16(messagesLen))
    buf = append(buf, messagesLenBuf...)

    for _, msg := range r.messages {
        msgLen := len(msg)
        lenBuf := make([]byte, 4)
        binary.BigEndian.PutUint32(lenBuf, uint32(msgLen))
        buf = append(buf, lenBuf...)
        buf = append(buf, msg...)
    }

    // 序列化addresses
    addressesLen := len(r.addresses)
    addressesLenBuf := make([]byte, 2)
    binary.BigEndian.PutUint16(addressesLenBuf, uint16(addressesLen))
    buf = append(buf, addressesLenBuf...)

    for _, addr := range r.addresses {
        addrBytes := []byte(addr)
        addrLen := len(addrBytes)
        lenBuf := make([]byte, 2)
        binary.BigEndian.PutUint16(lenBuf, uint16(addrLen))
        buf = append(buf, lenBuf...)
        buf = append(buf, addrBytes...)
    }

    // 序列化paths
    pathsLen := len(r.paths)
    pathsLenBuf := make([]byte, 2)
    binary.BigEndian.PutUint16(pathsLenBuf, uint16(pathsLen))
    buf = append(buf, pathsLenBuf...)

    for _, path := range r.paths {
        pathBytes := []byte(path)
        pathLen := len(pathBytes)
        lenBuf := make([]byte, 2)
        binary.BigEndian.PutUint16(lenBuf, uint16(pathLen))
        buf = append(buf, lenBuf...)
        buf = append(buf, pathBytes...)
    }

    return buf, nil
}

func (r *rpcBatchMessage) Unmarshal(data []byte) error {
    if len(data) < 2 {
        return errors.New("invalid data length")
    }

    offset := 0

    // 解析names
    namesLen := binary.BigEndian.Uint16(data[offset : offset+2])
    offset += 2

    r.names = make([]string, 0, namesLen)
    for i := 0; i < int(namesLen); i++ {
        if offset+2 > len(data) {
            return errors.New("invalid name length")
        }
        nameLen := binary.BigEndian.Uint16(data[offset : offset+2])
        offset += 2

        if offset+int(nameLen) > len(data) {
            return errors.New("invalid name data")
        }
        r.names = append(r.names, string(data[offset:offset+int(nameLen)]))
        offset += int(nameLen)
    }

    // 解析messages
    if offset+2 > len(data) {
        return errors.New("invalid messages length")
    }
    messagesLen := binary.BigEndian.Uint16(data[offset : offset+2])
    offset += 2

    r.messages = make([][]byte, 0, messagesLen)
    for i := 0; i < int(messagesLen); i++ {
        if offset+4 > len(data) {
            return errors.New("invalid message length")
        }
        msgLen := binary.BigEndian.Uint32(data[offset : offset+4])
        offset += 4

        if offset+int(msgLen) > len(data) {
            return errors.New("invalid message data")
        }
        r.messages = append(r.messages, data[offset:offset+int(msgLen)])
        offset += int(msgLen)
    }

    // 解析addresses
    if offset+2 > len(data) {
        return errors.New("invalid addresses length")
    }
    addressesLen := binary.BigEndian.Uint16(data[offset : offset+2])
    offset += 2

    r.addresses = make([]string, 0, addressesLen)
    for i := 0; i < int(addressesLen); i++ {
        if offset+2 > len(data) {
            return errors.New("invalid address length")
        }
        addrLen := binary.BigEndian.Uint16(data[offset : offset+2])
        offset += 2

        if offset+int(addrLen) > len(data) {
            return errors.New("invalid address data")
        }
        r.addresses = append(r.addresses, string(data[offset:offset+int(addrLen)]))
        offset += int(addrLen)
    }

    // 解析paths
    if offset+2 > len(data) {
        return errors.New("invalid paths length")
    }
    pathsLen := binary.BigEndian.Uint16(data[offset : offset+2])
    offset += 2

    r.paths = make([]string, 0, pathsLen)
    for i := 0; i < int(pathsLen); i++ {
        if offset+2 > len(data) {
            return errors.New("invalid path length")
        }
        pathLen := binary.BigEndian.Uint16(data[offset : offset+2])
        offset += 2

        if offset+int(pathLen) > len(data) {
            return errors.New("invalid path data")
        }
        r.paths = append(r.paths, string(data[offset:offset+int(pathLen)]))
        offset += int(pathLen)
    }

    return nil
}
