package messagecodecs

import (
	"fmt"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

func PipeResultEncoder() serialization.MessageEncoder {
	return serialization.MessageEncoderFN(func(writer *serialization.Writer, message any) error {
		m := message.(*vivid.PipeResult)
		if err := writer.Write(m.Message).Err(); err != nil {
			return err
		}

		var errorCode int32
		var errorMessage string
		switch err := m.Error.(type) {
		case nil:
		case *vivid.Error:
			errorCode = err.GetCode()
			errorMessage = err.GetMessage()
		default:
			errorCode = vivid.ErrorException.GetCode()
			errorMessage = vivid.ErrorException.With(err).GetMessage()
		}

		return writer.Write(
			m.Id,                    // PipeID
			errorCode, errorMessage, // 错误码和错误消息
		).Err()
	})
}

func PipeResultDecoder() serialization.MessageDecoder {
	return serialization.MessageDecoderFN(func(reader *serialization.Reader, message any) error {
		m := message.(*vivid.PipeResult)
		if err := reader.Read(&m.Message); err != nil {
			return err
		}

		var errorCode int32
		var errorMessage string
		if err := reader.Read(&errorCode, &errorMessage); err != nil {
			return err
		}

		if errorCode != 0 {
			var foundError = vivid.QueryError(errorCode)
			if foundError == nil {
				foundError = vivid.ErrorException.With(fmt.Errorf("error code %d not found, message: %s", errorCode, errorMessage))
				m.Error = foundError
				return nil
			}

			if errorMessage != "" && errorMessage != foundError.GetMessage() {
				m.Error = foundError.WithMessage(errorMessage)
			}
		}
		return nil
	})
}
