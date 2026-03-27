package libs

import (
	"sync"

	sdklibhttp "github.com/kyverno/sdk/cel/libs/http"
)

var (
	httpMockResponses   map[string]interface{}
	httpMockResponsesMu sync.RWMutex
)

// SetHTTPMockResponses sets the map used by CEL http.Get()/http.Post() during offline tests.
// Keys may be plain URLs or "METHOD:url" composite keys for method-specific matching.
func SetHTTPMockResponses(responses map[string]interface{}) {
	httpMockResponsesMu.Lock()
	defer httpMockResponsesMu.Unlock()
	httpMockResponses = responses
}

// GetHTTPContext returns an http.ContextInterface for CEL evaluation.
// When mock responses are configured, returns a context that serves mocks
// for known URLs and falls through to real HTTP for unknown URLs.
func GetHTTPContext() sdklibhttp.ContextInterface {
	httpMockResponsesMu.RLock()
	mock := httpMockResponses
	httpMockResponsesMu.RUnlock()
	if len(mock) > 0 {
		return &mockHTTPContext{
			responses: mock,
			fallback:  sdklibhttp.NewHTTP(nil),
		}
	}
	return sdklibhttp.NewHTTP(nil)
}

type mockHTTPContext struct {
	responses map[string]interface{}
	fallback  sdklibhttp.ContextInterface
}

func (m *mockHTTPContext) Get(url string, headers map[string]string) (any, error) {
	if body, ok := m.findResponse("GET", url); ok {
		return body, nil
	}
	return m.fallback.Get(url, headers)
}

func (m *mockHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	if body, ok := m.findResponse("POST", url); ok {
		return body, nil
	}
	return m.fallback.Post(url, data, headers)
}

func (m *mockHTTPContext) Client(_ string) (sdklibhttp.ContextInterface, error) {
	return m, nil
}

// findResponse looks up a mock response: try "METHOD:url" first, then plain "url".
// The returned body is wrapped with statusCode to match the SDK response format.
func (m *mockHTTPContext) findResponse(method, url string) (any, bool) {
	if body, ok := m.responses[method+":"+url]; ok {
		return body, true
	}
	if body, ok := m.responses[url]; ok {
		return body, true
	}
	return nil, false
}
