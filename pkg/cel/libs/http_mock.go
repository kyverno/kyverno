package libs

import (
	"sync"

	sdklibhttp "github.com/kyverno/sdk/cel/libs/http"
)

var (
	httpMockResponses   map[string]interface{}
	httpMockResponsesMu sync.RWMutex
)

// SetHTTPMockResponses sets per-URL (or METHOD:url) mock bodies for CEL http in offline tests; nil clears.
func SetHTTPMockResponses(responses map[string]interface{}) {
	httpMockResponsesMu.Lock()
	defer httpMockResponsesMu.Unlock()
	httpMockResponses = responses
}

// GetHTTPMockResponsesForTesting returns the current mock map (tests only).
func GetHTTPMockResponsesForTesting() map[string]interface{} {
	httpMockResponsesMu.RLock()
	defer httpMockResponsesMu.RUnlock()
	return httpMockResponses
}

// NewMockAwareHTTPContext wraps real HTTP CEL context; mocked URLs skip the network.
func NewMockAwareHTTPContext(real sdklibhttp.ContextInterface) sdklibhttp.ContextInterface {
	return &mockAwareHTTPContext{real: real}
}

type mockAwareHTTPContext struct {
	real sdklibhttp.ContextInterface
}

func (m *mockAwareHTTPContext) Get(url string, headers map[string]string) (any, error) {
	httpMockResponsesMu.RLock()
	var body any
	var ok bool
	if len(httpMockResponses) > 0 {
		body, ok = findMockResponse(httpMockResponses, "GET", url)
	}
	httpMockResponsesMu.RUnlock()
	if ok {
		return body, nil
	}
	return m.real.Get(url, headers)
}

func (m *mockAwareHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	httpMockResponsesMu.RLock()
	var body any
	var ok bool
	if len(httpMockResponses) > 0 {
		body, ok = findMockResponse(httpMockResponses, "POST", url)
	}
	httpMockResponsesMu.RUnlock()
	if ok {
		return body, nil
	}
	return m.real.Post(url, data, headers)
}

func (m *mockAwareHTTPContext) Client(caBundle string) (sdklibhttp.ContextInterface, error) {
	inner, err := m.real.Client(caBundle)
	if err != nil {
		return nil, err
	}
	return &mockAwareHTTPContext{real: inner}, nil
}

func findMockResponse(responses map[string]interface{}, method, url string) (any, bool) {
	if body, ok := responses[method+":"+url]; ok {
		return body, true
	}
	if body, ok := responses[url]; ok {
		return body, true
	}
	return nil, false
}
