package messages

var (
	_ InternalMessage = (*OnLaunch)(nil)
)

const (
	OnLaunchMessageType uint32 = iota + 1
)

var internalMessageFactory = map[uint32]func() InternalMessage{
	OnLaunchMessageType: func() InternalMessage { return &OnLaunch{} },
}

func NewInternalMessage(messageType uint32) InternalMessage {
	return internalMessageFactory[messageType]()
}

type OnLaunch struct{}

func (m *OnLaunch) MessageType() uint32 {
	return OnLaunchMessageType
}

func (m *OnLaunch) Read(reader *Reader) error {
	return nil
}

func (m *OnLaunch) Write(writer *Writer) error {
	return nil
}
