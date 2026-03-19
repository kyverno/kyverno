package libs

import (
	"fmt"
	"sync"

	sdklibhttp "github.com/kyverno/sdk/cel/libs/http"
)

var (
	httpMockResponses   map[string]interface{}
	httpMockResponsesMu sync.RWMutex
)

// SetHTTPMockResponses sets the map of URL -> response body used by CEL http.Get()
// when evaluating policies offline
func SetHTTPMockResponses(responses map[string]interface{}) {
	httpMockResponsesMu.Lock()
	defer httpMockResponsesMu.Unlock()
	httpMockResponses = responses
}

// GetHTTPContext returns an http.ContextInterface for CEL.
func GetHTTPContext() sdklibhttp.ContextInterface {
	httpMockResponsesMu.RLock()
	mock := httpMockResponses
	httpMockResponsesMu.RUnlock()
	if len(mock) > 0 {
		return &mockHTTPContext{responses: mock}
	}
	return sdklibhttp.NewHTTP(nil)
}

// mockHTTPContext implements github.com/kyverno/sdk/cel/libs/http.ContextInterface
// and returns static responses for URLs present in the map.
type mockHTTPContext struct {
	responses map[string]interface{}
}

func (m *mockHTTPContext) Get(url string, _ map[string]string) (any, error) {
	if body, ok := m.responses[url]; ok {
		return body, nil
	}
	return nil, fmt.Errorf("http.Get: no mock response for URL %q", url)
}

func (m *mockHTTPContext) Post(url string, _ any, _ map[string]string) (any, error) {
	if body, ok := m.responses[url]; ok {
		return body, nil
	}
	return nil, fmt.Errorf("http.Post: no mock response for URL %q", url)
}

func (m *mockHTTPContext) Client(_ string) (sdklibhttp.ContextInterface, error) {
	return m, nil
}
