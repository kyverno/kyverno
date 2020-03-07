package policystatus

import "sync"

type keyToMutex struct {
	mu    sync.RWMutex
	keyMu map[string]*sync.RWMutex
}

func newKeyToMutex() *keyToMutex {
	return &keyToMutex{
		mu:    sync.RWMutex{},
		keyMu: make(map[string]*sync.RWMutex),
	}
}

func (k *keyToMutex) Get(key string) *sync.RWMutex {
	k.mu.Lock()
	defer k.mu.Unlock()
	mutex := k.keyMu[key]
	if mutex == nil {
		mutex = &sync.RWMutex{}
		k.keyMu[key] = mutex
	}

	return mutex
}
