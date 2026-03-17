package store

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestNewMockGCtxStore(t *testing.T) {
	mocks := []v1alpha1.MockGlobalContextEntry{
		{
			Name: "entry-1",
			Data: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"name": "deploy-1"},
				},
			},
		},
		{
			Name: "entry-2",
			Data: "simple-string-value",
		},
	}
	store := NewMockGCtxStore(mocks)

	entry, ok := store.Get("entry-1")
	assert.True(t, ok)
	assert.NotNil(t, entry)
	data, err := entry.Get("")
	assert.NoError(t, err)
	assert.Equal(t, mocks[0].Data, data)

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
	e := &mockEntry{data: "test"}
	e.Stop()
}

func TestBuildMockAPICallURLIndex(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		index := buildMockAPICallURLIndex(nil)
		assert.Nil(t, index)
	})

	t.Run("empty input", func(t *testing.T) {
		index := buildMockAPICallURLIndex([]v1alpha1.MockAPICallResponse{})
		assert.Nil(t, index)
	})

	t.Run("with entries", func(t *testing.T) {
		mocks := []v1alpha1.MockAPICallResponse{
			{
				URLPath: "https://service.example.com/api/check",
				Response: v1alpha1.MockResponse{
					StatusCode: 200,
					Body: map[string]interface{}{
						"allowed": true,
					},
				},
			},
			{
				URLPath: "https://service.example.com/api/other",
				Response: v1alpha1.MockResponse{
					StatusCode: 200,
					Body:       "plain-string",
				},
			},
		}
		index := buildMockAPICallURLIndex(mocks)
		assert.Len(t, index, 2)

		body, ok := index["https://service.example.com/api/check"]
		assert.True(t, ok)
		assert.Equal(t, map[string]interface{}{"allowed": true}, body)

		body, ok = index["https://service.example.com/api/other"]
		assert.True(t, ok)
		assert.Equal(t, "plain-string", body)

		_, ok = index["https://missing.example.com"]
		assert.False(t, ok)
	})
}

func TestStoreSetGetMockData(t *testing.T) {
	s := &Store{}

	apiMocks := []v1alpha1.MockAPICallResponse{
		{URLPath: "https://example.com/api", Response: v1alpha1.MockResponse{Body: "data"}},
	}
	s.SetMockAPICallResponses(apiMocks)
	assert.Equal(t, apiMocks, s.GetMockAPICallResponses())

	gceMocks := []v1alpha1.MockGlobalContextEntry{
		{Name: "entry", Data: "value"},
	}
	s.SetMockGlobalContextEntries(gceMocks)
	assert.Equal(t, gceMocks, s.GetMockGlobalContextEntries())
}
