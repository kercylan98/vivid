package remoting

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	errFrameTooLarge        = errors.New("frame data length exceeds limit")
	errFrameControlTooLarge = errors.New("frame control length exceeds limit")
	errFrameShortWrite      = errors.New("short frame write")
)

type FrameType uint32

// 控制帧类型：0=握手，1=心跳，2=关闭，3=常规数据
const (
	FrameCtrlHandshake FrameType = iota
	FrameCtrlHeartbeat
	FrameCtrlClose
	FrameCtrlData
)

const frameHeaderSize = 12

const (
	maxFrameControlLen  = 4 * 1024        // 4KB
	maxFrameDataLen     = 4 * 1024 * 1024 // 4MB
	maxHandshakeDataLen = 1024
)

type Frame struct {
	Type        FrameType
	ControlData []byte
	Data        []byte
}

type FrameReadLimits struct {
	MaxControlLen uint32
	MaxDataLen    uint32
}

var (
	heartbeatFrameBytes = NewHeartbeatFrame().Bytes()
	closeFrameBytes     = NewCloseFrame().Bytes()
)

func NewHandshakeFrame(advertiseAddr string) Frame {
	return Frame{Type: FrameCtrlHandshake, Data: []byte(advertiseAddr)}
}

func NewHeartbeatFrame() Frame {
	return Frame{Type: FrameCtrlHeartbeat}
}

func NewCloseFrame() Frame {
	return Frame{Type: FrameCtrlClose}
}

func NewDataFrame(data []byte) Frame {
	return Frame{Type: FrameCtrlData, Data: data}
}

func (f Frame) Bytes() []byte {
	dataLen := uint32(len(f.Data))
	ctrlLen := uint32(len(f.ControlData))
	buf := make([]byte, frameHeaderSize+len(f.ControlData)+len(f.Data))
	binary.BigEndian.PutUint32(buf[0:4], uint32(f.Type))
	binary.BigEndian.PutUint32(buf[4:8], ctrlLen)
	binary.BigEndian.PutUint32(buf[8:12], dataLen)
	copy(buf[frameHeaderSize:frameHeaderSize+ctrlLen], f.ControlData)
	copy(buf[frameHeaderSize+ctrlLen:], f.Data)
	return buf
}

func (f Frame) WriteTo(writer io.Writer) (int64, error) {
	_, err := writeFull(writer, f.Bytes())
	return 0, err
}

func ReadFrame(reader io.Reader, header []byte, limits FrameReadLimits) (frame Frame, err error) {
	if len(header) < frameHeaderSize {
		header = make([]byte, frameHeaderSize)
	}
	if _, err = io.ReadFull(reader, header[:frameHeaderSize]); err != nil {
		return Frame{}, err
	}
	ctrlType := FrameType(binary.BigEndian.Uint32(header[0:4]))
	ctrlLen := binary.BigEndian.Uint32(header[4:8])
	dataLen := binary.BigEndian.Uint32(header[8:12])
	if limits.MaxControlLen > 0 && ctrlLen > limits.MaxControlLen {
		return Frame{}, errFrameControlTooLarge
	}
	if limits.MaxDataLen > 0 && dataLen > limits.MaxDataLen {
		return Frame{}, errFrameTooLarge
	}
	frame.Type = ctrlType
	if ctrlLen > 0 {
		frame.ControlData = make([]byte, ctrlLen)
		if _, err = io.ReadFull(reader, frame.ControlData); err != nil {
			return Frame{}, err
		}
	}
	if dataLen > 0 {
		frame.Data = make([]byte, dataLen)
		if _, err = io.ReadFull(reader, frame.Data); err != nil {
			return Frame{}, err
		}
	}
	return frame, nil
}

func writeFull(writer io.Writer, data []byte) (written int, err error) {
	for written < len(data) {
		n, writeErr := writer.Write(data[written:])
		written += n
		if writeErr != nil {
			return written, writeErr
		}
		if n == 0 {
			return written, errFrameShortWrite
		}
	}
	return written, nil
}
