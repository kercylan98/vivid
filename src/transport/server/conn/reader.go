package conn

import (
	"bytes"
	"github.com/kercylan98/vivid/pkg/vivid"
	"log"
)

var _ vivid.ConnReader = (*reader)(nil)

type reader struct {
	options vivid.ConnReaderOptionsFetcher
	buffer  *bytes.Buffer
	decoder vivid.Decoder
	conn    vivid.Conn
}

func (r *reader) Read(c chan<- vivid.Envelope) {
	var err error
	var lengthBytes = make([]byte, 4)
	var packetBytes = make([]byte, 2048)
	var packetLength int32
	var envelope vivid.Envelope
	var n int

	for {

		// 读取数据包长度
		if packetLength == 0 {
			n, err = r.conn.Read(lengthBytes)
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
		n, err = r.conn.Read(packetBytes)
		if err != nil {
			break
		}
		r.buffer.Write(packetBytes[:n])

		// 缓冲区数据长度达到数据包长度，处理数据包
		if int32(r.buffer.Len()) == packetLength {
			envelope, err = r.decoder.Decode()
			r.buffer.Reset()
			if err != nil {
				log.Println("Error decoding envelope:", err)
				break
			}

			c <- envelope
		}

	}

	if err = r.conn.Close(); err != nil {
		log.Println("Error closing connection:", err)
		return
	}
}
