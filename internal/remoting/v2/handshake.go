package remoting

import (
	"errors"
	"net"
	"time"
)

const handshakeTimeout = 10 * time.Second

var errExpectedHandshakeFrame = errors.New("remoting: expected handshake frame")

type Handshake struct {
	AdvertiseAddr string
}

func (h *Handshake) Send(conn net.Conn) error {
	return NewHandshakeFrame(h.AdvertiseAddr).WriteTo(conn)
}

func (h *Handshake) Wait(conn net.Conn) (err error) {
	if err = conn.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return err
	}
	defer func() {
		if resetErr := conn.SetReadDeadline(time.Time{}); resetErr != nil {
			err = errors.Join(err, resetErr)
		}
	}()

	header := make([]byte, frameHeaderSize)
	frame, err := ReadFrame(conn, header, FrameReadLimits{
		MaxControlLen: maxFrameControlLen,
		MaxDataLen:    maxHandshakeDataLen,
	})
	if err != nil {
		return err
	}
	if frame.Type != FrameCtrlHandshake {
		return errExpectedHandshakeFrame
	}
	h.AdvertiseAddr = string(frame.Data)
	return nil
}
