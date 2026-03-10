package gossipmessages

import (
	"time"

	"github.com/kercylan98/vivid/internal/gossip/internal/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/internal/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/internal/versionvector"
	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	messages.RegisterInternalMessage[*Ping]("Ping", onPingReader, onPingWriter)
	messages.RegisterInternalMessage[*Pong]("Pong", onPongReader, onPongWriter)
}

func NewPing(info *endpoint.Information, memberList *memberlist.MemberList, versionVector *versionvector.VersionVector) *Ping {
	return &Ping{
		Timestamp:  time.Now(),
		Info:       info,
		MemberList: memberList,
		Version:    versionVector,
	}
}

type Ping struct {
	Timestamp  time.Time                    // 发送 Ping 请求的时间
	Info       *endpoint.Information        // 发送 Ping 请求的节点信息
	MemberList *memberlist.MemberList       // 该节点所知的其他节点成员列表
	Version    *versionvector.VersionVector // 该节点版本向量
}

func onPingReader(message any, reader *messages.Reader, codec messages.Codec) error {
	p := message.(*Ping)
	var timestamp int64
	if err := reader.ReadInto(&timestamp); err != nil {
		return err
	}
	p.Timestamp = time.Unix(0, timestamp)
	return reader.ReadInto(&p.Info, &p.MemberList, &p.Version)
}

func onPingWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	p := message.(*Ping)
	return writer.WriteFrom(p.Timestamp.UnixNano(), p.Info, p.MemberList, p.Version)
}

func NewPong(info *endpoint.Information, memberList *memberlist.MemberList, versionVector *versionvector.VersionVector) *Pong {
	return &Pong{
		Timestamp:  time.Now(),
		Info:       info,
		MemberList: memberList,
		Version:    versionVector,
	}
}

type Pong struct {
	Timestamp  time.Time                    // 响应 Ping 请求的时间
	Info       *endpoint.Information        // 响应 Ping 请求的节点信息
	MemberList *memberlist.MemberList       // 该节点所知的其他节点成员列表
	Version    *versionvector.VersionVector // 该节点版本向量
}

func onPongReader(message any, reader *messages.Reader, codec messages.Codec) error {
	p := message.(*Pong)
	var timestamp int64
	if err := reader.ReadInto(&timestamp); err != nil {
		return err
	}
	p.Timestamp = time.Unix(0, timestamp)
	return reader.ReadInto(&p.Info, &p.MemberList, &p.Version)
}

func onPongWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	p := message.(*Pong)
	return writer.WriteFrom(p.Timestamp.UnixNano(), p.Info, p.MemberList, p.Version)
}
