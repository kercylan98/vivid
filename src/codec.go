package vivid

import (
	"bytes"
	"encoding/gob"
)

type Codec interface {
	Encode(v *Envelope) ([]byte, error)
	Decode(data []byte) (*Envelope, error)
}

type CodecProvider interface {
	Provide() Codec
}

type CodecProviderFn func() Codec

func (f CodecProviderFn) Provide() Codec {
	return f()
}

func newGobCodec() Codec {
	codec := &gobCodec{
		encoderBuf: new(bytes.Buffer),
		decoderBuf: new(bytes.Buffer),
	}
	codec.encoder = gob.NewEncoder(codec.encoderBuf)
	codec.decoder = gob.NewDecoder(codec.decoderBuf)
	return codec
}

type gobCodec struct {
	encoderBuf *bytes.Buffer
	decoderBuf *bytes.Buffer
	encoder    *gob.Encoder
	decoder    *gob.Decoder
}

func (c *gobCodec) Encode(v *Envelope) ([]byte, error) {
	defer c.encoderBuf.Reset()

	if err := c.encoder.Encode(v); err != nil {
		return nil, err
	}
	var data = make([]byte, c.encoderBuf.Len())
	copy(data, c.encoderBuf.Bytes())
	return data, nil
}

func (c *gobCodec) Decode(data []byte) (*Envelope, error) {
	c.decoderBuf.Write(data)
	defer c.decoderBuf.Reset()

	var envelope = newEnvelope()
	if err := c.decoder.Decode(envelope); err != nil {
		return nil, err
	}
	return envelope, nil
}
