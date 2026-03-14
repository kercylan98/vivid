package remoting

import (
	"encoding/binary"
	"errors"
	"io"
)

var errFrameTooLarge = errors.New("remoting: frame data length exceeds limit")

// 控制帧类型：0=握手，1=心跳，2=关闭，3=常规数据
const (
	FrameCtrlHandshake uint32 = 0
	FrameCtrlHeartbeat uint32 = 1
	FrameCtrlClose     uint32 = 2
	FrameCtrlData      uint32 = 3
)

const frameHeaderSize = 12

// writeFrame 向 conn 写入一帧：4B 控制类型 + 4B 控制数据长度 + 4B 常规数据长度 + 控制数据 + 常规数据
func writeFrame(conn io.Writer, ctrlType uint32, ctrlData, data []byte) error {
	ctrlLen := uint32(len(ctrlData))
	dataLen := uint32(len(data))
	buf := make([]byte, frameHeaderSize+len(ctrlData)+len(data))
	binary.BigEndian.PutUint32(buf[0:4], ctrlType)
	binary.BigEndian.PutUint32(buf[4:8], ctrlLen)
	binary.BigEndian.PutUint32(buf[8:12], dataLen)
	copy(buf[frameHeaderSize:frameHeaderSize+ctrlLen], ctrlData)
	copy(buf[frameHeaderSize+ctrlLen:], data)
	_, err := conn.Write(buf)
	return err
}

var (
	heartbeatFrameBytes = func() []byte {
		b := make([]byte, frameHeaderSize)
		binary.BigEndian.PutUint32(b[0:4], FrameCtrlHeartbeat)
		return b
	}()
	closeFrameBytes = func() []byte {
		b := make([]byte, frameHeaderSize)
		binary.BigEndian.PutUint32(b[0:4], FrameCtrlClose)
		return b
	}()
)

// dataFrameBytes 构造类型 3（常规数据）帧的完整字节：12 字节头 + data
func dataFrameBytes(data []byte) []byte {
	dataLen := uint32(len(data))
	buf := make([]byte, frameHeaderSize+len(data))
	binary.BigEndian.PutUint32(buf[0:4], FrameCtrlData)
	binary.BigEndian.PutUint32(buf[4:8], 0)
	binary.BigEndian.PutUint32(buf[8:12], dataLen)
	copy(buf[frameHeaderSize:], data)
	return buf
}

// readFrame 从 reader 读完整一帧，返回控制类型、控制数据、常规数据
func readFrame(reader io.Reader, header []byte) (ctrlType uint32, ctrlData, data []byte, err error) {
	return readFrameWithMaxDataLen(reader, header, 0)
}

// readFrameWithMaxDataLen 与 readFrame 相同，但限制 data 长度不超过 maxDataLen（0 表示不限制）。
func readFrameWithMaxDataLen(reader io.Reader, header []byte, maxDataLen uint32) (ctrlType uint32, ctrlData, data []byte, err error) {
	if len(header) < frameHeaderSize {
		header = make([]byte, frameHeaderSize)
	}
	if _, err = io.ReadFull(reader, header[:frameHeaderSize]); err != nil {
		return 0, nil, nil, err
	}
	ctrlType = binary.BigEndian.Uint32(header[0:4])
	ctrlLen := binary.BigEndian.Uint32(header[4:8])
	dataLen := binary.BigEndian.Uint32(header[8:12])
	if maxDataLen > 0 && dataLen > maxDataLen {
		return 0, nil, nil, errFrameTooLarge
	}
	if ctrlLen > 0 {
		ctrlData = make([]byte, ctrlLen)
		if _, err = io.ReadFull(reader, ctrlData); err != nil {
			return 0, nil, nil, err
		}
	}
	if dataLen > 0 {
		data = make([]byte, dataLen)
		if _, err = io.ReadFull(reader, data); err != nil {
			return 0, nil, nil, err
		}
	}
	return ctrlType, ctrlData, data, nil
}
