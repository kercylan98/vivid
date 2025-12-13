package remoting

import (
	"net"

	"github.com/kercylan98/vivid/internal/messages"
)

type Handshake struct {
	AdvertiseAddr string // 广告地址
}

func (h *Handshake) Send(conn net.Conn) error {
	writer := messages.NewWriterFromPool()
	if err := writer.WriteFrom(h.AdvertiseAddr); err != nil {
		return err
	}
	data := writer.Bytes()

	messages.ReleaseWriterToPool(writer)
	_, err := conn.Write(data)
	return err
}

func (h *Handshake) Wait(conn net.Conn) error {
	var buf = make([]byte, 4096)
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
