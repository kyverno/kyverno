package webhook

import (
	"strings"
	"sync"
)

type Recorder struct {
	lock       sync.Mutex
	data       map[string]bool
	NotifyChan chan string
}

type StateRecorder interface {
	Ready(string) (bool, bool)
	Record(string)
	Reset()
}

func NewStateRecorder(notifyChan chan string) StateRecorder {
	return &Recorder{
		data:       make(map[string]bool),
		NotifyChan: notifyChan,
	}
}

func (s *Recorder) Ready(key string) (bool, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	ready, ok := s.data[key]
	return ready, ok
}

func (s *Recorder) Record(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data[key] = true
	if s.NotifyChan != nil {
		s.NotifyChan <- key
	}
}

func (s *Recorder) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()
	for d := range s.data {
		s.data[d] = false
	}
}

// BuildRecorderKey builds policy key in kind/name format
func BuildRecorderKey(policyType, name string) string {
	switch policyType {
	case ValidatingPolicyType:
		return ValidatingPolicyType + "/" + name
	case ImageValidatingPolicyType:
		return ImageValidatingPolicyType + "/" + name
	case MutatingPolicyType:
		return MutatingPolicyType + "/" + name
	case GeneratingPolicyType:
		return GeneratingPolicyType + "/" + name
	}
	return ""
}

// ParseRecorderKey parses policy key in kind/name format
func ParseRecorderKey(key string) (policyType, name string) {
	vars := strings.Split(key, "/")
	if len(vars) < 2 {
		return "", ""
	}
	return vars[0], vars[1]
}
