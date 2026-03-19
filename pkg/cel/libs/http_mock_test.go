package libs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetHTTPMockResponses_GetHTTPContext(t *testing.T) {
	// No mock set -> GetHTTPContext returns default (non-mock)
	SetHTTPMockResponses(nil)
	ctx := GetHTTPContext()
	assert.NotNil(t, ctx)

	// Set mock -> GetHTTPContext returns mock implementation
	mock := map[string]interface{}{
		"https://example.com/api": map[string]interface{}{"allowed": true},
	}
	SetHTTPMockResponses(mock)
	ctx = GetHTTPContext()
	assert.NotNil(t, ctx)

	body, err := ctx.Get("https://example.com/api", nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"allowed": true}, body)

	_, err = ctx.Get("https://other.com", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no mock response")

	// Clear mock
	SetHTTPMockResponses(nil)
	ctx = GetHTTPContext()
	assert.NotNil(t, ctx)
}

func TestMockHTTPContext_Post(t *testing.T) {
	mock := map[string]interface{}{
		"https://post.example.com": "ok",
	}
	ctx := &mockHTTPContext{responses: mock}

	body, err := ctx.Post("https://post.example.com", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "ok", body)

	_, err = ctx.Post("https://missing.example.com", nil, nil)
	assert.Error(t, err)
}

func TestMockHTTPContext_Client(t *testing.T) {
	ctx := &mockHTTPContext{responses: map[string]interface{}{}}
	got, err := ctx.Client("")
	assert.NoError(t, err)
	assert.Equal(t, ctx, got)
}
