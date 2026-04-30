package libs

import (
	sdklibhttp "github.com/kyverno/sdk/cel/libs/http"
)

// NewMockAwareHTTPContext wraps a real HTTP CEL context with a per-instance mock map.
// Mocked URLs are served from the map; everything else falls through to the real context.
func NewMockAwareHTTPContext(real sdklibhttp.ContextInterface, mocks map[string]interface{}) sdklibhttp.ContextInterface {
	return &mockAwareHTTPContext{real: real, mocks: mocks}
}

type mockAwareHTTPContext struct {
	real  sdklibhttp.ContextInterface
	mocks map[string]interface{}
}

func (m *mockAwareHTTPContext) Get(url string, headers map[string]string) (any, error) {
	if body, ok := findMockResponse(m.mocks, "GET", url); ok {
		return body, nil
	}
	return m.real.Get(url, headers)
}

func (m *mockAwareHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	if body, ok := findMockResponse(m.mocks, "POST", url); ok {
		return body, nil
	}
	return m.real.Post(url, data, headers)
}

func (m *mockAwareHTTPContext) Client(caBundle string) (sdklibhttp.ContextInterface, error) {
	inner, err := m.real.Client(caBundle)
	if err != nil {
		return nil, err
	}
	return &mockAwareHTTPContext{real: inner, mocks: m.mocks}, nil
}

func findMockResponse(responses map[string]interface{}, method, url string) (any, bool) {
	if len(responses) == 0 {
		return nil, false
	}
	if body, ok := responses[method+":"+url]; ok {
		return body, true
	}
	if body, ok := responses[url]; ok {
		return body, true
	}
	return nil, false
}
