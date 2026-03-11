package remoting

import (
	"net"
	"time"
)

type Handshake struct {
	AdvertiseAddr string
}

func (h *Handshake) Send(conn net.Conn) error {
	return writeFrame(conn, FrameCtrlHandshake, nil, []byte(h.AdvertiseAddr))
}

func (h *Handshake) Wait(conn net.Conn) error {
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	header := make([]byte, frameHeaderSize)
	ctrlType, _, data, err := readFrame(conn, header)
	if err != nil {
		return err
	}
	if ctrlType != FrameCtrlHandshake {
		h.AdvertiseAddr = ""
		return nil
	}
	h.AdvertiseAddr = string(data)
	return nil
}
