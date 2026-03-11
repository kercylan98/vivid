package remoting

import (
	"encoding/binary"
	"net"
	"time"
)

type Handshake struct {
	AdvertiseAddr string // 广告地址
}

func (h *Handshake) Send(conn net.Conn) error {
	var advertiseAddrLength = len(h.AdvertiseAddr)

	var buf = make([]byte, 4+advertiseAddrLength)
	// 写入广告地址长度
	binary.BigEndian.PutUint32(buf, uint32(advertiseAddrLength))
	// 写入广告地址
	copy(buf[4:], h.AdvertiseAddr)

	_, err := conn.Write(buf)
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

	advertiseAddrLength := binary.BigEndian.Uint32(buf)
	h.AdvertiseAddr = string(buf[4 : 4+advertiseAddrLength])

	return nil
}
