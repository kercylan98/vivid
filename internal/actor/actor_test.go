package actor

import (
	"errors"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
)

var ExportNewReplaceEnvelop = newReplacedEnvelop

func init() {
	vivid.RegisterCustomMessage[*TestRemoteMessage]("TestRemoteMessage",
		func(message any, reader *messages.Reader, codec messages.Codec) (err error) {
			var testRemoteMessage TestRemoteMessage
			err = reader.ReadInto(&testRemoteMessage.Text, &testRemoteMessage.WriteCodecFail, &testRemoteMessage.ReadCodecFail)
			if err != nil {
				return err
			}
			if testRemoteMessage.ReadCodecFail {
				return errors.New("codec read fail")
			}
			return nil
		},
		func(message any, writer *messages.Writer, codec messages.Codec) (err error) {
			testRemoteMessage := message.(*TestRemoteMessage)
			if testRemoteMessage.WriteCodecFail {
				return errors.New("codec write fail")
			}
			return writer.WriteFrom(testRemoteMessage.Text, testRemoteMessage.WriteCodecFail, testRemoteMessage.ReadCodecFail)
		},
	)
}

func NewUselessActor() vivid.Actor {
	return vivid.ActorFN(func(ctx vivid.ActorContext) {})
}

// TestRemoteMessage 测试远程消息
type TestRemoteMessage struct {
	Text           string // 消息文本
	WriteCodecFail bool   // 是否编解码失败
	ReadCodecFail  bool   // 是否编解码失败
}
