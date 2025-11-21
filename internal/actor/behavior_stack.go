package actor

import "github.com/kercylan98/vivid"

func NewBehaviorStack(behaviors ...vivid.Behavior) *BehaviorStack {
	return &BehaviorStack{
		behaviors: behaviors,
	}
}

type BehaviorStack struct {
	behaviors []vivid.Behavior
}

func (s *BehaviorStack) Peak() vivid.Behavior {
	if len(s.behaviors) == 0 {
		return nil
	}
	return s.behaviors[len(s.behaviors)-1]
}

func (s *BehaviorStack) Push(behavior vivid.Behavior) {
	s.behaviors = append(s.behaviors, behavior)
}

func (s *BehaviorStack) Pop() vivid.Behavior {
	if len(s.behaviors) == 0 {
		return nil
	}
	behavior := s.behaviors[len(s.behaviors)-1]
	s.behaviors = s.behaviors[:len(s.behaviors)-1]
	return behavior
}

func (s *BehaviorStack) Clear() {
	s.behaviors = nil
}

func (s *BehaviorStack) Len() int {
	return len(s.behaviors)
}

func (s *BehaviorStack) IsEmpty() bool {
	return len(s.behaviors) == 0
}
