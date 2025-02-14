package vivid

import (
	"context"
	"errors"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/internal/protobuf/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"sync"
	"time"
)

const (
	// remoteStreamLimit 对同一个远端最多允许同时存在的远程流数量
	remoteStreamLimit = 5
)

func newRemoteStreamManager(manager processManager) *remoteStreamManager {
	return &remoteStreamManager{
		processManager: manager,
		streams:        make(map[Addr][]remoteStream),
	}
}

type remoteStreamManager struct {
	processManager processManager
	streams        map[Addr][]remoteStream
	rw             sync.RWMutex
}

// unbindRemoteStream 解绑远程流
func (r *remoteStreamManager) unbindRemoteStream(addr Addr, stream remoteStream) {
	r.rw.Lock()
	defer r.rw.Unlock()

	if streams, ok := r.streams[addr]; ok {
		for i, s := range streams {
			if s == stream {
				r.streams[addr] = append(streams[:i], streams[i+1:]...)
				return
			}
		}
	}
}

// bindRemoteStream 绑定已有的远程流
func (r *remoteStreamManager) bindRemoteStream(addr Addr, stream remoteStream) {
	r.rw.Lock()
	defer r.rw.Unlock()

	r.streams[addr] = append(r.streams[addr], stream)
}

// loadOrInitClientRemoteStream 加载或初始化客户端远程流，该函数会尝试从缓存中获取远程流，如果不存在则创建一个新的远程流
func (r *remoteStreamManager) loadOrInitClientRemoteStream(addr Addr) (remoteStream, error) {
	// 采用读锁获取目标，避免每次获取都加写锁，需要使用双重校验来确保不会重复创建
	r.rw.RLock()
	index := time.Now().UnixMilli() % remoteStreamLimit
	if streams, ok := r.streams[addr]; ok {
		// 随机选择一个远程流
		if index < int64(len(streams)) {
			r.rw.RUnlock()
			return streams[index], nil
		}
	}
	r.rw.RUnlock()
	if stream, err := r.createRemoteStream(addr); err != nil {
		r.processManager.logger().Warn("remote", log.String("event", "dial"), log.Any("info", "retrying on a continuous basis"), log.Any("err", err))
		return nil, err
	} else {
		return stream, nil
	}
}

// createRemoteStream 创建一个远程流
func (r *remoteStreamManager) createRemoteStream(addr Addr) (remoteStream, error) {
	r.rw.Lock()
	defer r.rw.Unlock()

	// 双重校验，防止重复创建
	streams, exist := r.streams[addr]
	if exist && len(streams) == remoteStreamLimit {
		return streams[time.Now().UnixMilli()%remoteStreamLimit], nil
	}

	// 创建客户端并打开连接
	stream, err := r.openRemoteStream(addr)
	if err != nil {
		return nil, err
	}

	// 发起握手
	if err = r.sendRemoteStreamHandshake(stream); err != nil {
		stream.close()
		return nil, err
	}

	// 激活
	if handshake, err := r.waitRemoteStreamHandshake(stream); err != nil {
		stream.close()
		return nil, err
	} else {
		stream.bindAddr(handshake.Address)
	}

	// 记录
	streams = append(streams, stream)

	go r.startListenRemoteStreamMessage(stream)
	return stream, nil
}

// sendRemoteStreamHandshake 由客户端发送握手消息
func (r *remoteStreamManager) sendRemoteStreamHandshake(stream remoteStream) error {
	if !stream.isOpener() {
		panic("sendRemoteStreamHandshake is only allowed for opener(Indicates the connection that opens the stream, not the connection that the server listens to)")
	}
	msg := &protobuf.Message{
		MessageType: &protobuf.Message_Handshake_{Handshake: &protobuf.Message_Handshake{Address: r.processManager.getHost()}},
	}
	if err := stream.Send(msg); err != nil {
		return err
	}
	return nil
}

// waitRemoteStreamHandshake 等待握手消息的到来并回复
//   - 第一条收到的消息必须是来自客户端发起的握手消息，并且回复握手消息
func (r *remoteStreamManager) waitRemoteStreamHandshake(stream remoteStream) (*protobuf.Message_Handshake, error) {
	message, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	handshake := message.GetHandshake()
	if handshake == nil {
		return nil, errors.New("waitRemoteStreamHandshake message is expected")
	}

	addr := r.processManager.getHost()
	if handshake.Address == addr {
		return nil, errors.New("loop-back connection is not allowed")
	}

	handshake.Address = addr
	return handshake, stream.Send(message)
}

// startListenRemoteStreamMessage 开始监听远程流消息
func (r *remoteStreamManager) startListenRemoteStreamMessage(stream remoteStream) {
	defer func() {
		stream.close()
	}()

	var codec = r.processManager.getCodecProvider().Provide()
	var message *protobuf.Message
	var err error
	for {
		message, err = stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}

		r.onReceiveRemoteStreamMessage(stream, codec, message)
	}
}

// openRemoteStream 打开 GRPC 远程流
func (r *remoteStreamManager) openRemoteStream(addr string) (remoteStream, error) {
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := protobuf.NewVividServiceClient(cc)
	server, err := client.OpenMessageStream(context.Background())
	if err != nil {
		return nil, err
	}

	return newRemoteStream(r, server), nil
}

// onReceiveRemoteStreamMessage 处理远程流消息
func (r *remoteStreamManager) onReceiveRemoteStreamMessage(stream remoteStream, codec Codec, message *protobuf.Message) {
	switch m := message.GetMessageType().(type) {
	case *protobuf.Message_Batch_:
		r.onStreamBatchMessage(stream, codec, m.Batch)
	case *protobuf.Message_Farewell_:
		r.onStreamFarewellMessage(stream, m.Farewell)
	}
}

func (r *remoteStreamManager) onStreamBatchMessage(stream remoteStream, codec Codec, batch *protobuf.Message_Batch) {
	var receiverCache = make(map[Path]Process)
	for i, messageBytes := range batch.Messages {
		var envelope, err = codec.Decode(messageBytes)
		if err != nil {
			// TODO: 不应该直接 panic
			panic(err)
		}

		receiver := envelope.GetReceiver()
		path := receiver.GetPath()

		// 加载缓存
		process, exist := receiverCache[path]
		if exist {
			process.Send(envelope)
			continue
		}

		process, _ = r.processManager.getProcess(receiver)
		process.Send(envelope)

		// 建立缓存
		if i != len(batch.Messages)-1 {
			if _, exist := receiverCache[path]; !exist {
				receiverCache[path] = process
			}
		}

	}
}

func (r *remoteStreamManager) onStreamFarewellMessage(stream remoteStream, farewell *protobuf.Message_Farewell) {

}
