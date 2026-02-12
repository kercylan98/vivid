package store

import (
	"sync"
	"time"
)

const maxRecent = 5

// DeathLetterItem 单条死信摘要，用于控制台展示。
type DeathLetterItem struct {
	Time        string `json:"time"`        // RFC3339
	Sender      string `json:"sender"`       // 发送者路径或地址
	Receiver    string `json:"receiver"`     // 接收者路径或地址
	MessageType string `json:"messageType"` // 消息类型，如 *messages.PingMessage
}

// EventItem 单条事件摘要，用于控制台展示。
type EventItem struct {
	Time    string `json:"time"`    // RFC3339
	Type    string `json:"type"`    // 事件类型名，如 ActorLaunched
	Summary string `json:"summary"` // 简短描述
}

// RecentStore 保存最近 N 条死信与事件，新条目在索引 0，向下推挤。并发安全。
type RecentStore struct {
	mu           sync.RWMutex
	deathLetters []DeathLetterItem
	events       []EventItem
}

// NewRecentStore 创建容量为 maxRecent 的 store。
func NewRecentStore() *RecentStore {
	return &RecentStore{
		deathLetters: make([]DeathLetterItem, 0, maxRecent),
		events:       make([]EventItem, 0, maxRecent),
	}
}

// AddDeathLetter 在头部插入一条死信，超过 maxRecent 的从尾部丢弃。
func (s *RecentStore) AddDeathLetter(sender, receiver, messageType string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := DeathLetterItem{
		Time:        at.UTC().Format(time.RFC3339),
		Sender:      sender,
		Receiver:    receiver,
		MessageType: messageType,
	}
	s.deathLetters = prependDeathLetter(s.deathLetters, item, maxRecent)
}

func prependDeathLetter(d []DeathLetterItem, item DeathLetterItem, max int) []DeathLetterItem {
	out := make([]DeathLetterItem, 0, max+1)
	out = append(out, item)
	out = append(out, d...)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func prependEvent(e []EventItem, item EventItem, max int) []EventItem {
	out := make([]EventItem, 0, max+1)
	out = append(out, item)
	out = append(out, e...)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// AddEvent 在头部插入一条事件，超过 maxRecent 的从尾部丢弃。
func (s *RecentStore) AddEvent(typ, summary string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := EventItem{
		Time:    at.UTC().Format(time.RFC3339),
		Type:    typ,
		Summary: summary,
	}
	s.events = prependEvent(s.events, item, maxRecent)
}

// GetDeathLetters 返回最近死信列表，新在前；只读，调用方不要修改。
func (s *RecentStore) GetDeathLetters() []DeathLetterItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DeathLetterItem, len(s.deathLetters))
	copy(out, s.deathLetters)
	return out
}

// GetEvents 返回最近事件列表，新在前；只读。
func (s *RecentStore) GetEvents() []EventItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EventItem, len(s.events))
	copy(out, s.events)
	return out
}
