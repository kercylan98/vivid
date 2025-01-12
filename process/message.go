package main

const (
	UserMessage MessageType = iota
	SystemMessage
)

type MessageType = uint8

// Message 是一个抽象概念，它代表了一个消息
type Message = any
