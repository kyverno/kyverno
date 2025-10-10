package data

import (
	"fmt"
	"sync"
)

var (
	processor     *CRDProcessor
	processorOnce sync.Once
)

func InjectProcessor(p *CRDProcessor) {
	processorOnce.Do(func() {
		processor = p
	})
}

func GetProcessor() (*CRDProcessor, error) {
	if processor == nil {
		return nil, fmt.Errorf("CRDProcessor not initialized.")
	}
	return processor, nil
}
