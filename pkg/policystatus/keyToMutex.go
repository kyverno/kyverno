package policystatus

import "sync"

// keyToMutex allows status to be updated
//for different policies at the same time
//while ensuring the status for same policies
//are updated one at a time.
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
