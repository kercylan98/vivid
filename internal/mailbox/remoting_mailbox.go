package mailbox

import (
	"fmt"
	"net"
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting/serialize"
	"github.com/kercylan98/vivid/internal/utils"
	"golang.org/x/sync/singleflight"
)

const (
	poolSize = 10
)

var (
	_ vivid.Mailbox = &RemotingMailbox{}
)

func NewRemotingMailbox(advertiseAddress string) *RemotingMailbox {
	return &RemotingMailbox{
		advertiseAddress: advertiseAddress,
		singleflight:     &singleflight.Group{},
	}
}

type RemotingMailbox struct {
	advertiseAddress string
	connections      [poolSize]net.Conn
	connectionLock   sync.RWMutex
	singleflight     *singleflight.Group
}

func (m *RemotingMailbox) Enqueue(envelop vivid.Envelop) {
	sender := envelop.Sender()
	slot := utils.Fnv32aHash(sender.GetPath()) % poolSize

	// 获取连接，如果没有则创建一个
	m.connectionLock.RLock()
	conn := m.connections[slot]
	m.connectionLock.RUnlock()
	if conn == nil {
		conn, err, _ := m.singleflight.Do(fmt.Sprint(slot), func() (any, error) {
			m.connectionLock.Lock()
			defer m.connectionLock.Unlock()

			conn, err := net.Dial("tcp", m.advertiseAddress)
			if err != nil {
				return nil, err
			}

			// 发送客户端握手协议
			handshakeProtocol := &Handshake{
				AdvertiseAddr: m.advertiseAddress,
			}
			if err := handshakeProtocol.Send(conn); err != nil {
				return nil, err
			}

			// 等待服务端握手协议
			if err := handshakeProtocol.Wait(conn); err != nil {
				return nil, err
			}

			m.connections[slot] = conn
			return conn, nil
		})
		if err != nil {
			panic(err)
		}
		conn = conn.(net.Conn)
	}

	// 发送消息
	data, err := serialize.SerializeEnvelopWithRemoting(envelop)
	if err != nil {
		panic(err)
	}
	_, err = conn.Write(data)
	if err != nil {
		panic(err)
	}
}
