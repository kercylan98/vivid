package actor

import "github.com/kercylan98/vivid"

var emptyBehavior = func(ctx vivid.ActorContext) {}

func NewBehaviorStack(behaviors ...vivid.Behavior) *BehaviorStack {
	return &BehaviorStack{
		behaviors: behaviors,
	}
}

type BehaviorStack struct {
	behaviors []vivid.Behavior
}

func (s *BehaviorStack) Peek() vivid.Behavior {
	if len(s.behaviors) == 0 {
		return nil
	}
	return s.behaviors[len(s.behaviors)-1]
}

func (s *BehaviorStack) Push(behavior vivid.Behavior) *BehaviorStack {
	s.behaviors = append(s.behaviors, behavior)
	return s
}

func (s *BehaviorStack) Pop() vivid.Behavior {
	if len(s.behaviors) == 0 {
		return nil
	}
	behavior := s.behaviors[len(s.behaviors)-1]
	s.behaviors = s.behaviors[:len(s.behaviors)-1]
	return behavior
}

func (s *BehaviorStack) Clear() *BehaviorStack {
	s.behaviors = nil
	return s
}

func (s *BehaviorStack) Len() int {
	return len(s.behaviors)
}

func (s *BehaviorStack) IsEmpty() bool {
	return len(s.behaviors) == 0
}
