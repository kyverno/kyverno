package webhook

import (
	"strings"
	"sync"
)

type Recorder struct {
	lock       sync.Mutex
	data       map[string]bool
	notifyChan chan string
}

type StateRecorder interface {
	Ready(string) (bool, bool)
	Record(string)
	Reset()
	NotifyChannel() <-chan string
}

func NewStateRecorder(notifyChan chan string) StateRecorder {
	return &Recorder{
		data:       make(map[string]bool),
		notifyChan: notifyChan,
	}
}

func (s *Recorder) NotifyChannel() <-chan string {
	return s.notifyChan
}

func (s *Recorder) Ready(key string) (bool, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	ready, ok := s.data[key]
	return ready, ok
}

func (s *Recorder) Record(key string) {
	s.lock.Lock()
	s.data[key] = true
	s.lock.Unlock()

	if s.notifyChan != nil {
		s.notifyChan <- key
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
func BuildRecorderKey(policyType, name, namespace string) string {
	switch policyType {
	case ValidatingPolicyType:
		return ValidatingPolicyType + "/" + name
	case NamespacedValidatingPolicyType:
		return NamespacedValidatingPolicyType + "/" + name + "+" + namespace
	case ImageValidatingPolicyType:
		return ImageValidatingPolicyType + "/" + name
	case NamespacedImageValidatingPolicyType:
		return NamespacedImageValidatingPolicyType + "/" + name + "+" + namespace
	case MutatingPolicyType:
		return MutatingPolicyType + "/" + name
	case GeneratingPolicyType:
		return GeneratingPolicyType + "/" + name
	}
	return ""
}

// ParseRecorderKey parses policy key in kind/name format
func ParseRecorderKey(key string) (policyType, name, namespace string) {
	vars := strings.Split(key, "/")
	if len(vars) < 2 {
		return "", "", ""
	}

	parts := strings.Split(vars[1], "+")
	if len(parts) == 2 {
		return vars[0], parts[0], parts[1]
	}

	return vars[0], vars[1], ""
}
