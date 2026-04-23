/*
Copyright 2025 The Kyverno Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package libs

import (
	"sync"

	sdklibhttp "github.com/kyverno/sdk/cel/libs/http"
)

var (
	httpMockResponses   map[string]interface{}
	httpMockResponsesMu sync.RWMutex
)

// SetHTTPMockResponses sets the map used by CEL http.Get() during offline tests.
// Keys may be plain URLs ("https://example.com/data") or "METHOD:url" composite
// keys for method-specific matching ("GET:https://example.com/data").
// Pass nil to clear all mocks after a test case.
func SetHTTPMockResponses(responses map[string]interface{}) {
	httpMockResponsesMu.Lock()
	defer httpMockResponsesMu.Unlock()
	httpMockResponses = responses
}

// GetHTTPMockResponsesForTesting returns the current mock map. Only for use in tests.
func GetHTTPMockResponsesForTesting() map[string]interface{} {
	httpMockResponsesMu.RLock()
	defer httpMockResponsesMu.RUnlock()
	return httpMockResponses
}

// NewMockAwareHTTPContext wraps a real http.ContextInterface so that mock responses
// (set via SetHTTPMockResponses) are checked at call time — not at compile time when
// the CEL environment is built. This preserves the real context's SSRF blocklist and
// namespace toggle for any URL that is not mocked.
//
// Usage in compilers:
//
//	http.Lib(
//	    http.Context{ContextInterface: libs.NewMockAwareHTTPContext(compiler.NewLazyCELHTTPContext(namespace))},
//	    http.Latest(),
//	)
func NewMockAwareHTTPContext(real sdklibhttp.ContextInterface) sdklibhttp.ContextInterface {
	return &mockAwareHTTPContext{real: real}
}

// mockAwareHTTPContext delegates to mock responses when configured, otherwise
// falls through to the wrapped real HTTP context.
type mockAwareHTTPContext struct {
	real sdklibhttp.ContextInterface
}

func (m *mockAwareHTTPContext) Get(url string, headers map[string]string) (any, error) {
	httpMockResponsesMu.RLock()
	mock := httpMockResponses
	httpMockResponsesMu.RUnlock()
	if len(mock) > 0 {
		if body, ok := findMockResponse(mock, "GET", url); ok {
			return body, nil
		}
	}
	return m.real.Get(url, headers)
}

func (m *mockAwareHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	httpMockResponsesMu.RLock()
	mock := httpMockResponses
	httpMockResponsesMu.RUnlock()
	if len(mock) > 0 {
		if body, ok := findMockResponse(mock, "POST", url); ok {
			return body, nil
		}
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

// findMockResponse looks up a mock response: tries "METHOD:url" first, then plain "url".
func findMockResponse(responses map[string]interface{}, method, url string) (any, bool) {
	if body, ok := responses[method+":"+url]; ok {
		return body, true
	}
	if body, ok := responses[url]; ok {
		return body, true
	}
	return nil, false
}
