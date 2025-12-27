package remoting

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

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
		backoff:           utils.NewExponentialBackoffWithDefault(100*time.Millisecond, 3*time.Second),
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
	backoff           *utils.ExponentialBackoff
}

func (m *Mailbox) Pause() {
	// 远程邮箱不考虑，该邮箱仅作为向外投递消息的中转通道
}

func (m *Mailbox) Resume() {
	// 远程邮箱不考虑，该邮箱仅作为向外投递消息的中转通道
}

func (m *Mailbox) IsPaused() bool {
	return false
}

func (m *Mailbox) Enqueue(envelop vivid.Envelop) {
	sender := envelop.Sender()
	slot := utils.Fnv32aHash(sender.GetPath()) % poolSize

	// 尝试获取有效连接，如果不存在或已关闭则创建新连接
	var conn *tcpConnectionActor
	_, err := m.backoff.Forever(func() (abort bool, err error) {
		// 获取连接，如果没有则创建一个
		m.connectionLock.RLock()
		conn = m.connections[slot]
		m.connectionLock.RUnlock()

		// 如果连接已关闭，设置为 nil，后续会重新创建连接
		if conn != nil && conn.Closed() {
			// 清理已关闭的连接
			m.connectionLock.Lock()
			if m.connections[slot] == conn {
				m.connections[slot] = nil
			}
			m.connectionLock.Unlock()
			conn = nil
		}

		if conn == nil {
			c, err, _ := m.sf.Do(fmt.Sprint(slot), func() (any, error) {
				m.connectionLock.Lock()
				defer m.connectionLock.Unlock()

				// Double-check: 在获取锁后再次检查连接是否已存在且有效
				if existingConn := m.connections[slot]; existingConn != nil && !existingConn.Closed() {
					return existingConn, nil
				}

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
				// 创建连接失败，继续重试
				return false, err
			}
			conn = c.(*tcpConnectionActor)
		}

		// 序列化消息，如果失败应当直接中断，重试毫无意义
		data, err := serialize.EncodeEnvelopWithRemoting(m.codec, envelop)
		if err != nil {
			return true, err
		}
		// 写入消息长度
		lengthBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
		data = append(lengthBuf, data...)

		// 在使用连接前再次检查连接是否有效
		if conn.Closed() {
			// 连接在使用前被关闭，清理并重试
			m.connectionLock.Lock()
			if m.connections[slot] == conn {
				m.connections[slot] = nil
			}
			m.connectionLock.Unlock()
			conn = nil
			return false, fmt.Errorf("connection closed before write")
		}

		// 发送消息
		_, err = conn.Write(data)
		if err != nil {
			// 写入失败，可能是连接已关闭，清理连接并重试
			m.connectionLock.Lock()
			if m.connections[slot] == conn {
				m.connections[slot] = nil
			}
			m.connectionLock.Unlock()
			conn = nil
			return false, err
		}
		return true, nil
	})

	if err != nil {
		// 如果是因为 abort=true 导致的退出（如序列化失败），直接 panic
		// 注意：Forever 只有在 abort=true 或 err=nil 时才会退出
		// 如果 err != nil，说明是 abort=true 导致的退出（如序列化失败）
		panic(err)
	}
	// err == nil 说明操作成功（成功时返回 true, nil）
}
