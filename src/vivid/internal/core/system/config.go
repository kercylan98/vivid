package system

import (
    "time"

    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/wasteland/src/wasteland"
)

type Config struct {
    LoggerProvider           log.Provider                // 日志提供者
    Address                  wasteland.Address           // 网络地址
    CodecProvider            wasteland.CodecProvider     // 编解码器提供者
    RPCMessageBuilder        wasteland.RPCMessageBuilder // RPC 消息构建器
    GuardDefaultRestartLimit int                         // 默认重启次数限制
    TimingWheelTick          time.Duration               // 定时器滴答时间
    TimingWheelSize          int                         // 定时器大小
    StopTimeout              time.Duration               // 系统停止超时时间，0表示无超时
    PoisonStopTimeout        time.Duration               // 优雅停止超时时间，0表示无超时
}
