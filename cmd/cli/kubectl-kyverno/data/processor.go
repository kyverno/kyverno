package data

import (
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

func GetProcessor() *crdProcessor {
	return processor 
}
