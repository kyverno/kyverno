package webhook

import "sync"

type stateRecorder struct {
	lock sync.Mutex
	data map[string]bool
}

type StateRecorder interface {
	Ready(string) bool
	Record(string)
	Reset()
}

func NewStateRecorder() StateRecorder {
	return &stateRecorder{
		data: make(map[string]bool),
	}
}

func (s *stateRecorder) Ready(key string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.data[key]
}

func (s *stateRecorder) Record(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data[key] = true
}

func (s *stateRecorder) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data = make(map[string]bool)
}
