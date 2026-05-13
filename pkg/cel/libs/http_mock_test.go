package libs

import (
	"testing"

	sdklibhttp "github.com/kyverno/sdk/cel/libs/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHTTPContext struct {
	called bool
}

func (s *stubHTTPContext) Get(_ string, _ map[string]string) (any, error) {
	s.called = true
	return map[string]interface{}{"real": true}, nil
}

func (s *stubHTTPContext) Post(_ string, _ any, _ map[string]string) (any, error) {
	s.called = true
	return map[string]interface{}{"real": true}, nil
}

func (s *stubHTTPContext) Client(_ string) (sdklibhttp.ContextInterface, error) {
	return s, nil
}

func TestMockAwareHTTPContext_Post_MockHit(t *testing.T) {
	mocks := map[string]interface{}{
		"POST:https://example.com/submit": map[string]interface{}{"id": "42", "statusCode": 201},
	}

	stub := &stubHTTPContext{}
	ctx := NewMockAwareHTTPContext(stub, mocks)

	result, err := ctx.Post("https://example.com/submit", map[string]interface{}{"k": "v"}, nil)
	require.NoError(t, err)
	assert.False(t, stub.called)
	m := result.(map[string]interface{})
	assert.Equal(t, "42", m["id"])
}

func TestMockAwareHTTPContext_Get_MockHit(t *testing.T) {
	mocks := map[string]interface{}{
		"https://example.com/data": map[string]interface{}{"allowed": true, "statusCode": 200},
	}

	stub := &stubHTTPContext{}
	ctx := NewMockAwareHTTPContext(stub, mocks)

	result, err := ctx.Get("https://example.com/data", nil)
	require.NoError(t, err)
	assert.False(t, stub.called)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["allowed"])
	assert.Equal(t, 200, m["statusCode"])
}

func TestMockAwareHTTPContext_Get_MockMiss_FallsThrough(t *testing.T) {
	mocks := map[string]interface{}{
		"https://example.com/known": map[string]interface{}{"ok": true, "statusCode": 200},
	}

	stub := &stubHTTPContext{}
	ctx := NewMockAwareHTTPContext(stub, mocks)

	result, err := ctx.Get("https://example.com/unknown", nil)
	require.NoError(t, err)
	assert.True(t, stub.called)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["real"])
}

func TestMockAwareHTTPContext_Get_NoMocks_FallsThrough(t *testing.T) {
	stub := &stubHTTPContext{}
	ctx := NewMockAwareHTTPContext(stub, nil)

	_, err := ctx.Get("https://example.com/any", nil)
	require.NoError(t, err)
	assert.True(t, stub.called)
}

func TestMockAwareHTTPContext_MethodKey_GET(t *testing.T) {
	mocks := map[string]interface{}{
		"GET:https://example.com/cfg": map[string]interface{}{"flag": "on", "statusCode": 200},
	}

	stub := &stubHTTPContext{}
	ctx := NewMockAwareHTTPContext(stub, mocks)

	result, err := ctx.Get("https://example.com/cfg", nil)
	require.NoError(t, err)
	assert.False(t, stub.called)
	m := result.(map[string]interface{})
	assert.Equal(t, "on", m["flag"])
}

func TestMockAwareHTTPContext_Client_WrapsInner(t *testing.T) {
	stub := &stubHTTPContext{}
	ctx := NewMockAwareHTTPContext(stub, nil)

	inner, err := ctx.Client("")
	require.NoError(t, err)
	_, ok := inner.(*mockAwareHTTPContext)
	assert.True(t, ok)
}

func TestFindMockResponse_MethodKeyPriority(t *testing.T) {
	responses := map[string]interface{}{
		"GET:https://example.com/url": "method-specific",
		"https://example.com/url":     "plain",
	}
	body, ok := findMockResponse(responses, "GET", "https://example.com/url")
	assert.True(t, ok)
	assert.Equal(t, "method-specific", body)
}

func TestFindMockResponse_PlainKeyFallback(t *testing.T) {
	responses := map[string]interface{}{
		"https://example.com/url": "plain",
	}
	body, ok := findMockResponse(responses, "GET", "https://example.com/url")
	assert.True(t, ok)
	assert.Equal(t, "plain", body)
}

func TestFindMockResponse_Miss(t *testing.T) {
	responses := map[string]interface{}{
		"https://other.com": "other",
	}
	_, ok := findMockResponse(responses, "GET", "https://example.com/url")
	assert.False(t, ok)
}

func TestFindMockResponse_NilMap(t *testing.T) {
	_, ok := findMockResponse(nil, "GET", "https://example.com/url")
	assert.False(t, ok)
}
