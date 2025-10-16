package registry

import (
	"fmt"

	"github.com/kercylan98/vivid/pkg/runtime"
	"github.com/kercylan98/vivid/pkg/serializer"
)

// newServerHandler 创建一个新的服务器处理器。
func newServerHandler(localAddress string, registry runtime.AddressingRegistry, serializer serializer.NameSerializer) *serverHandler {
	return &serverHandler{
		localAddress: localAddress,
		registry:     registry,
		serializer:   serializer,
	}
}

// serverHandler 处理服务器接收的连接。
type serverHandler struct {
	localAddress string
	registry     runtime.AddressingRegistry
	serializer   serializer.NameSerializer
}

// HandleConnection 处理单个连接。
// 该方法会阻塞直到连接关闭或发生错误。
func (h *serverHandler) HandleConnection(conn Connection) error {
	// 执行握手
	remoteAddress, err := h.performHandshake(conn)
	if err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	// 循环接收消息
	for {
		data, err := conn.Recv()
		if err != nil {
			return err
		}

		// 解码数据包
		packet := AcquirePacket()
		if err := packet.Decode(data); err != nil {
			ReleasePacket(packet)
			return fmt.Errorf("decode packet failed: %w", err)
		}

		// 处理数据包
		if err := h.handlePacket(remoteAddress, packet); err != nil {
			ReleasePacket(packet)
			// 处理错误但不中断连接
			continue
		}

		ReleasePacket(packet)
	}
}

// performHandshake 执行握手协议。
func (h *serverHandler) performHandshake(conn Connection) (string, error) {
	// 接收握手消息
	data, err := conn.Recv()
	if err != nil {
		return "", err
	}

	handshake := &Handshake{}
	if err := handshake.Decode(data); err != nil {
		return "", fmt.Errorf("decode handshake failed: %w", err)
	}

	// 发送握手响应
	response := NewHandshake(h.localAddress)
	responseData, err := response.Encode()
	if err != nil {
		return "", err
	}

	if err := conn.Send(responseData); err != nil {
		return "", err
	}

	return handshake.Address(), nil
}

// handlePacket 处理接收到的数据包。
func (h *serverHandler) handlePacket(remoteAddress string, packet *Packet) error {
	// 查找目标进程
	receiverID := runtime.NewProcessID(h.localAddress, packet.Receiver())
	process, err := h.registry.Find(&receiverID)
	if err != nil {
		return fmt.Errorf("process not found: %s, error: %w", packet.Receiver(), err)
	}

	// 反序列化消息
	message, err := h.serializer.Deserialize(packet.MessageName(), packet.MessageData())
	if err != nil {
		return fmt.Errorf("deserialize message failed: %w", err)
	}

	// 构建发送者 ID
	var senderID *runtime.ProcessID
	if packet.Sender() != "" {
		sid := runtime.NewProcessID(remoteAddress, packet.Sender())
		senderID = &sid
	}

	// 调用进程处理消息
	if err := process.OnMessage(senderID, packet.MessageType(), message); err != nil {
		return fmt.Errorf("process handle message failed: %w", err)
	}

	return nil
}
