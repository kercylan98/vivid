package server

import (
	"bytes"
	"github.com/kercylan98/vivid/core"
	"log"
	"net"
)

var _ core.Server = (*server)(nil)

type server struct {
	decoderBuilder   core.DecoderBuilder
	envelopeProvider core.EnvelopeProvider
	envelopeChannel  chan core.Envelope
}

func (srv *server) GetEnvelopeChannel() <-chan core.Envelope {
	return srv.envelopeChannel
}

func (srv *server) Serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go srv.handle(conn)
	}
}

func (srv *server) handle(conn net.Conn) {
	var err error
	var buffer bytes.Buffer
	var decoder = srv.decoderBuilder.Build(&buffer, srv.envelopeProvider)
	var lengthBytes = make([]byte, 4)
	var packetBytes = make([]byte, 2048)
	var packetLength int32
	var envelope core.Envelope
	var n int

	for {

		// 读取数据包长度
		if packetLength == 0 {
			n, err = conn.Read(lengthBytes)
			if err != nil {
				break
			}

			// 读取到的字节数不足 4 个字节，异常无效数据包
			if n != 4 {
				break
			}

			packetLength = int32(lengthBytes[0])<<24 | int32(lengthBytes[1])<<16 | int32(lengthBytes[2])<<8 | int32(lengthBytes[3])
		}

		// 读取数据包并写入缓冲区
		n, err = conn.Read(packetBytes)
		if err != nil {
			break
		}
		buffer.Write(packetBytes[:n])

		// 缓冲区数据长度达到数据包长度，处理数据包
		if int32(buffer.Len()) == packetLength {
			envelope, err = decoder.Decode()
			buffer.Reset()
			if err != nil {
				log.Println("Error decoding envelope:", err)
				break
			}

			srv.envelopeChannel <- envelope
		}

	}

	if err = conn.Close(); err != nil {
		log.Println("Error closing connection:", err)
		return
	}
}
