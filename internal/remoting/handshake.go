package remoting

import (
	"net"
	"time"

	"github.com/kercylan98/vivid/internal/messages"
)

type Handshake struct {
	AdvertiseAddr string // 广告地址
}

func (h *Handshake) Send(conn net.Conn) error {
	writer := messages.NewWriterFromPool()
	defer messages.ReleaseWriterToPool(writer)
	if err := writer.WriteFrom(h.AdvertiseAddr); err != nil {
		return err
	}
	data := writer.Bytes()
	if err := conn.SetWriteDeadline(time.Now().Add(time.Second * 10)); err != nil {
		return err
	}

	_, err := conn.Write(data)
	return err
}

func (h *Handshake) Wait(conn net.Conn) error {
	var buf = make([]byte, 4096)
	if err := conn.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
		return err
	}

	if _, err := conn.Read(buf); err != nil {
		return err
	}
	reader := messages.NewReaderFromPool(buf)
	defer messages.ReleaseReaderToPool(reader)
	if err := reader.ReadInto(&h.AdvertiseAddr); err != nil {
		return err
	}
	return nil
}
