package libs

import (
	"testing"

	sdklibhttp "github.com/kyverno/sdk/cel/libs/http"
	"github.com/stretchr/testify/assert"
)

func TestSetHTTPMockResponses_GetHTTPContext(t *testing.T) {
	SetHTTPMockResponses(nil)
	ctx := GetHTTPContext()
	assert.NotNil(t, ctx)

	mock := map[string]interface{}{
		"https://example.com/api": map[string]interface{}{"allowed": true},
	}
	SetHTTPMockResponses(mock)
	ctx = GetHTTPContext()
	assert.NotNil(t, ctx)

	body, err := ctx.Get("https://example.com/api", nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"allowed": true}, body)

	SetHTTPMockResponses(nil)
	ctx = GetHTTPContext()
	assert.NotNil(t, ctx)
}

func TestMockHTTPContext_MethodSpecific(t *testing.T) {
	mock := map[string]interface{}{
		"GET:https://api.example.com/data":  map[string]interface{}{"read": true},
		"POST:https://api.example.com/data": map[string]interface{}{"created": true},
	}
	ctx := &mockHTTPContext{responses: mock, fallback: sdklibhttp.NewHTTP(nil)}

	body, err := ctx.Get("https://api.example.com/data", nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"read": true}, body)

	body, err = ctx.Post("https://api.example.com/data", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"created": true}, body)
}

func TestMockHTTPContext_AnyMethod(t *testing.T) {
	mock := map[string]interface{}{
		"https://api.example.com/any": "ok-for-any",
	}
	ctx := &mockHTTPContext{responses: mock, fallback: sdklibhttp.NewHTTP(nil)}

	body, err := ctx.Get("https://api.example.com/any", nil)
	assert.NoError(t, err)
	assert.Equal(t, "ok-for-any", body)

	body, err = ctx.Post("https://api.example.com/any", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "ok-for-any", body)
}

func TestMockHTTPContext_Client(t *testing.T) {
	ctx := &mockHTTPContext{responses: map[string]interface{}{}, fallback: sdklibhttp.NewHTTP(nil)}
	got, err := ctx.Client("")
	assert.NoError(t, err)
	assert.Equal(t, ctx, got)
}
