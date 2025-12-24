package remoting

import (
	"encoding/binary"
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
	_ vivid.Mailbox = &Mailbox{}
)

func newMailbox(advertiseAddress string, codec vivid.Codec, envelopHandler NetworkEnvelopHandler, actorLiaison vivid.ActorLiaison, remotingServerRef vivid.ActorRef) *Mailbox {
	return &Mailbox{
		advertiseAddress:  advertiseAddress,
		sf:                &singleflight.Group{},
		envelopHandler:    envelopHandler,
		actorLiaison:      actorLiaison,
		remotingServerRef: remotingServerRef,
		codec:             codec,
	}
}

type Mailbox struct {
	advertiseAddress  string
	connections       [poolSize]*tcpConnectionActor
	connectionLock    sync.RWMutex
	sf                *singleflight.Group
	envelopHandler    NetworkEnvelopHandler
	actorLiaison      vivid.ActorLiaison
	remotingServerRef vivid.ActorRef
	codec             vivid.Codec
}

func (m *Mailbox) Enqueue(envelop vivid.Envelop) {
	sender := envelop.Sender()
	slot := utils.Fnv32aHash(sender.GetPath()) % poolSize

	// 获取连接，如果没有则创建一个
	m.connectionLock.RLock()
	conn := m.connections[slot]
	m.connectionLock.RUnlock()
	if conn == nil {
		c, err, _ := m.sf.Do(fmt.Sprint(slot), func() (any, error) {
			m.connectionLock.Lock()
			defer m.connectionLock.Unlock()

			conn, err := net.Dial("tcp", m.advertiseAddress)
			if err != nil {
				return nil, err
			}

			tcpConn := newTCPConnectionActor(true, conn, m.advertiseAddress, m.codec, m.envelopHandler)
			if err = m.actorLiaison.Ask(m.remotingServerRef, tcpConn).Wait(); err != nil {
				closeErr := tcpConn.Close()
				if closeErr != nil {
					return nil, fmt.Errorf("%w, %s", err, closeErr)
				}
				return nil, err
			}

			m.connections[slot] = tcpConn
			return tcpConn, nil
		})
		if err != nil {
			panic(err)
		}
		conn = c.(*tcpConnectionActor)
	}

	// 序列化消息
	data, err := serialize.EncodeEnvelopWithRemoting(m.codec, envelop)
	if err != nil {
		panic(err)
	}
	// 写入消息长度
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
	data = append(lengthBuf, data...)
	// 发送消息
	if _, err = conn.Write(data); err != nil {
		panic(err)
	}
}
