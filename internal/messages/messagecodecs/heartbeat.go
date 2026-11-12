package messagecodecs

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

func HeartbeatEncoder() serialization.MessageEncoder {
	return serialization.MessageEncoderFN(func(writer *serialization.Writer, message any) error {
		m := message.(*vivid.Heartbeat)
		var refStr string
		if m.Ref != nil {
			refStr = m.Ref.String()
		}
		return writer.Write(refStr, m.Available).Err()
	})
}

func HeartbeatDecoder() serialization.MessageDecoder {
	return serialization.MessageDecoderFN(func(reader *serialization.Reader, message any) error {
		var refStr string
		m := message.(*vivid.Heartbeat)
		if err := reader.Read(&refStr, &m.Available); err != nil {
			return err
		}

		if refStr != "" {
			ref, err := actorRefParser(refStr)
			if err != nil {
				return err
			}
			m.Ref = ref
		}
		return nil
	})
}
