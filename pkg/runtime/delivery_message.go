package runtime

type deliveryMessage struct {
	deliveryId uint64
	message    Message
}

func DeliveryMessage(deliveryId uint64, message Message) *deliveryMessage {
	return &deliveryMessage{
		deliveryId: deliveryId,
		message:    message,
	}
}

func ParseDeliveryMessage(m Message) (deliveryId uint64, message Message) {
	switch m := m.(type) {
	case *deliveryMessage:
		return m.deliveryId, m.message
	default:
		return 0, m
	}
}
