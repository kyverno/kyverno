package store

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewMockGCtxStore(t *testing.T) {
	raw1, err := json.Marshal(map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"name": "deploy-1"},
		},
	})
	assert.NoError(t, err)
	mocks := []v1alpha1.GlobalContextEntryValue{
		{Name: "entry-1", Data: runtime.RawExtension{Raw: raw1}},
		{Name: "entry-2", Data: runtime.RawExtension{Raw: []byte(`"simple-string-value"`)}},
	}
	store := NewMockGCtxStore(mocks)

	entry, ok := store.Get("entry-1")
	assert.True(t, ok)
	assert.NotNil(t, entry)
	data, err := entry.Get("")
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"items": []interface{}{map[string]interface{}{"name": "deploy-1"}}}, data)

	entry, ok = store.Get("entry-2")
	assert.True(t, ok)
	data, err = entry.Get("")
	assert.NoError(t, err)
	assert.Equal(t, "simple-string-value", data)

	entry, ok = store.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, entry)
}

func TestMockEntry_Stop(t *testing.T) {
	e := &mockEntry{data: runtime.RawExtension{}}
	e.Stop()
}

func TestBuildAPICallURLIndex(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		index, err := buildAPICallURLIndex(nil)
		assert.NoError(t, err)
		assert.Nil(t, index)
	})

	t.Run("empty input", func(t *testing.T) {
		index, err := buildAPICallURLIndex([]v1alpha1.APICallResponseEntry{})
		assert.NoError(t, err)
		assert.Nil(t, index)
	})

	t.Run("with entries no method", func(t *testing.T) {
		body1, _ := json.Marshal(map[string]interface{}{"allowed": true})
		mocks := []v1alpha1.APICallResponseEntry{
			{
				URL: "https://service.example.com/api/check",
				Response: v1alpha1.APICallResponse{
					StatusCode: 200,
					Body:       runtime.RawExtension{Raw: body1},
				},
			},
			{
				URL: "https://service.example.com/api/other",
				Response: v1alpha1.APICallResponse{
					StatusCode: 200,
					Body:       runtime.RawExtension{Raw: []byte(`"plain-string"`)},
				},
			},
		}
		index, err := buildAPICallURLIndex(mocks)
		assert.NoError(t, err)
		assert.Len(t, index, 2)

		body, ok := index["https://service.example.com/api/check"]
		assert.True(t, ok)
		assert.Equal(t, map[string]interface{}{"allowed": true}, body)

		body, ok = index["https://service.example.com/api/other"]
		assert.True(t, ok)
		assert.Equal(t, "plain-string", body)
	})

	t.Run("with method-specific entries", func(t *testing.T) {
		body1, _ := json.Marshal(map[string]interface{}{"read": true})
		body2, _ := json.Marshal(map[string]interface{}{"created": true})
		mocks := []v1alpha1.APICallResponseEntry{
			{
				URL:    "https://api.example.com/data",
				Method: "GET",
				Response: v1alpha1.APICallResponse{
					Body: runtime.RawExtension{Raw: body1},
				},
			},
			{
				URL:    "https://api.example.com/data",
				Method: "POST",
				Response: v1alpha1.APICallResponse{
					Body: runtime.RawExtension{Raw: body2},
				},
			},
		}
		index, err := buildAPICallURLIndex(mocks)
		assert.NoError(t, err)
		assert.Len(t, index, 2)

		body, ok := index["GET:https://api.example.com/data"]
		assert.True(t, ok)
		assert.Equal(t, map[string]interface{}{"read": true}, body)

		body, ok = index["POST:https://api.example.com/data"]
		assert.True(t, ok)
		assert.Equal(t, map[string]interface{}{"created": true}, body)
	})
}

func TestLookupMockResponse(t *testing.T) {
	index := map[string]interface{}{
		"https://any.example.com":            "any-method",
		"GET:https://specific.example.com":   "get-only",
		"POST:https://specific.example.com":  "post-only",
	}

	body, ok := lookupMockResponse(index, "GET", "https://any.example.com")
	assert.True(t, ok)
	assert.Equal(t, "any-method", body)

	body, ok = lookupMockResponse(index, "POST", "https://any.example.com")
	assert.True(t, ok)
	assert.Equal(t, "any-method", body)

	body, ok = lookupMockResponse(index, "GET", "https://specific.example.com")
	assert.True(t, ok)
	assert.Equal(t, "get-only", body)

	body, ok = lookupMockResponse(index, "POST", "https://specific.example.com")
	assert.True(t, ok)
	assert.Equal(t, "post-only", body)

	_, ok = lookupMockResponse(index, "GET", "https://missing.example.com")
	assert.False(t, ok)
}

func TestStoreSetGetData(t *testing.T) {
	s := &Store{}

	apiMocks := []v1alpha1.APICallResponseEntry{
		{URL: "https://example.com/api", Response: v1alpha1.APICallResponse{Body: runtime.RawExtension{Raw: []byte(`"data"`)}}},
	}
	s.SetAPICallResponses(apiMocks)
	assert.Equal(t, apiMocks, s.GetAPICallResponses())

	gceMocks := []v1alpha1.GlobalContextEntryValue{
		{Name: "entry", Data: runtime.RawExtension{Raw: []byte(`"value"`)}},
	}
	s.SetGlobalContextEntries(gceMocks)
	assert.Equal(t, gceMocks, s.GetGlobalContextEntries())
}
