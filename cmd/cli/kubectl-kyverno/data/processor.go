package data

import (
	"fmt"
	"sync"
)

var (
	processor     *crdProcessor
	processorOnce sync.Once
)

func InjectProcessor(p *crdProcessor) {
	processorOnce.Do(func() {
		processor = p
	})
}

func GetProcessor() (*crdProcessor, error) {
	if processor == nil {
		return nil, fmt.Errorf("crdProcessor not initialized.")
	}
	return processor, nil
}
