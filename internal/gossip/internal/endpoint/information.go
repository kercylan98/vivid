package endpoint

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/vividkit"
)

func init() {
	messages.RegisterInternalMessage[*Information]("Information", onInformationReader, onInformationWriter)
}

func NewInformation(actorRef vivid.ActorRef) *Information {
	return &Information{
		Status:   StatusUnknown,
		LastSeen: time.Now(),
		ActorRef: actorRef,
	}
}

// Information 端点信息
type Information struct {
	ActorRef vivid.ActorRef // 节点 Actor 引用
	Status   Status         // 节点状态
	LastSeen time.Time      // 最后一次心跳时间
}

func (i *Information) SetStatus(status Status) (changed bool, needGossip bool) {
	changed, needGossip = i.Status.CanTransitionTo(status)
	if changed {
		i.Status = status
	}
	return
}

func (i *Information) ID() string {
	return i.ActorRef.String()
}

func onInformationReader(message any, reader *messages.Reader, codec messages.Codec) error {
	i := message.(*Information)
	var actorRef string
	var status uint8
	var lastSeenUnixNano int64
	if err := reader.ReadInto(&actorRef, &status, &lastSeenUnixNano); err != nil {
		return err
	}
	ref, err := vividkit.ParseActorRef(actorRef)
	if err != nil {
		return err
	}
	i.ActorRef = ref
	i.Status = Status(status)
	i.LastSeen = time.Unix(0, lastSeenUnixNano)

	return nil
}

func onInformationWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	i := message.(*Information)
	return writer.WriteFrom(i.ActorRef.String(), uint8(i.Status), i.LastSeen.UnixNano())
}
