package data

import (
	"fmt"
	"sync"
)

var (
	processor     *crdProcessor
	processorOnce sync.Once
	procMu        sync.RWMutex
)

func InjectProcessor(p *crdProcessor) {
	processorOnce.Do(func() {
		procMu.Lock()
		defer procMu.Unlock()
		processor = p
	})
}

func GetProcessor() (*crdProcessor, error) {
	procMu.RLock()
	defer procMu.RUnlock()
	if processor == nil {
		return nil, fmt.Errorf("crdProcessor not initialized.")
	}
	return processor, nil
}
