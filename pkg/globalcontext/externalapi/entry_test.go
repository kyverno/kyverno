package externalapi

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/assert"
)

// mockJMESPathQuery implements jmespath.Query for testing
type mockJMESPathQuery struct {
	result any
	err    error
}

func (m *mockJMESPathQuery) Search(data any) (any, error) {
	return m.result, m.err
}

func TestEntry_Get_WithValidData(t *testing.T) {
	e := &entry{
		dataMap: map[string]any{
			"":            map[string]any{"key": "value"},
			"projection1": "projected-value",
		},
		err: nil,
	}

	tests := []struct {
		name       string
		projection string
		want       any
		wantErr    bool
	}{
		{
			name:       "get default projection",
			projection: "",
			want:       map[string]any{"key": "value"},
			wantErr:    false,
		},
		{
			name:       "get named projection",
			projection: "projection1",
			want:       "projected-value",
			wantErr:    false,
		},
		{
			name:       "get nonexistent projection",
			projection: "nonexistent",
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Get(tt.projection)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no data available")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEntry_Get_WithError(t *testing.T) {
	expectedErr := fmt.Errorf("api call failed")
	e := &entry{
		dataMap: map[string]any{},
		err:     expectedErr,
	}

	got, err := e.Get("")
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, got)
}

func TestEntry_Get_WithNilData(t *testing.T) {
	e := &entry{
		dataMap: map[string]any{},
		err:     nil,
	}

	got, err := e.Get("projection")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no data available")
	assert.Nil(t, got)
}

func TestEntry_Get_EmptyProjectionName(t *testing.T) {
	e := &entry{
		dataMap: map[string]any{
			"": map[string]any{"data": "test"},
		},
		err: nil,
	}

	got, err := e.Get("")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"data": "test"}, got)
}

func TestEntry_Stop_CallsStopFunction(t *testing.T) {
	stopCalled := false
	e := &entry{
		dataMap: make(map[string]any),
		stop: func() {
			stopCalled = true
		},
	}

	e.Stop()
	assert.True(t, stopCalled, "stop function should be called")
}

func TestEntry_Stop_WithNilStopFunction(t *testing.T) {
	e := &entry{
		dataMap: make(map[string]any),
		stop:    nil,
	}

	// Should panic when stop is nil
	assert.Panics(t, func() {
		e.Stop()
	}, "calling Stop with nil stop function should panic")
}

func TestEntry_SetData_WithValidJSON(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	jsonData := []byte(`{"name": "test", "value": 123}`)
	e.setData(jsonData, nil)

	assert.Nil(t, e.err, "error should be nil after successful setData")
	assert.NotNil(t, e.dataMap[""], "default projection should be set")

	data := e.dataMap[""].(map[string]any)
	assert.Equal(t, "test", data["name"])
	assert.Equal(t, float64(123), data["value"]) // JSON numbers are float64
}

func TestEntry_SetData_WithInvalidJSON(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	invalidJSON := []byte(`{invalid json}`)
	e.setData(invalidJSON, nil)

	assert.Error(t, e.err, "error should be set for invalid JSON")
	assert.Contains(t, e.err.Error(), "invalid character")
}

func TestEntry_SetData_WithError(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	expectedErr := fmt.Errorf("api call error")
	e.setData(nil, expectedErr)

	assert.Equal(t, expectedErr, e.err, "error should be set")
}

func TestEntry_SetData_WithNonByteData(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	// Pass a string instead of []byte
	e.setData("not a byte array", nil)

	assert.Error(t, e.err, "error should be set for non-byte data")
	assert.Contains(t, e.err.Error(), "data is not a byte array")
}

func TestEntry_SetData_WithProjections(t *testing.T) {
	mockQuery := &mockJMESPathQuery{
		result: "projected-result",
		err:    nil,
	}

	e := &entry{
		dataMap: make(map[string]any),
		err:     nil,
		projections: []store.Projection{
			{
				Name: "projection1",
				JP:   mockQuery,
			},
		},
	}

	jsonData := []byte(`{"key": "value"}`)
	e.setData(jsonData, nil)

	assert.Nil(t, e.err, "error should be nil")
	assert.NotNil(t, e.dataMap[""], "default projection should be set")
	assert.Equal(t, "projected-result", e.dataMap["projection1"], "named projection should be set")
}

func TestEntry_SetData_WithMultipleProjections(t *testing.T) {
	mockQuery1 := &mockJMESPathQuery{
		result: "result1",
		err:    nil,
	}
	mockQuery2 := &mockJMESPathQuery{
		result: "result2",
		err:    nil,
	}

	e := &entry{
		dataMap: make(map[string]any),
		err:     nil,
		projections: []store.Projection{
			{Name: "proj1", JP: mockQuery1},
			{Name: "proj2", JP: mockQuery2},
		},
	}

	jsonData := []byte(`{"data": "test"}`)
	e.setData(jsonData, nil)

	assert.Nil(t, e.err)
	assert.NotNil(t, e.dataMap[""])
	assert.Equal(t, "result1", e.dataMap["proj1"])
	assert.Equal(t, "result2", e.dataMap["proj2"])
}

func TestEntry_SetData_WithProjectionError(t *testing.T) {
	projectionErr := fmt.Errorf("jmespath query failed")
	mockQuery := &mockJMESPathQuery{
		result: nil,
		err:    projectionErr,
	}

	e := &entry{
		dataMap: make(map[string]any),
		err:     nil,
		projections: []store.Projection{
			{
				Name: "projection1",
				JP:   mockQuery,
			},
		},
	}

	jsonData := []byte(`{"key": "value"}`)
	e.setData(jsonData, nil)

	assert.Equal(t, projectionErr, e.err, "projection error should be set")
}

func TestEntry_SetData_EmptyJSON(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	jsonData := []byte(`{}`)
	e.setData(jsonData, nil)

	assert.Nil(t, e.err)
	assert.NotNil(t, e.dataMap[""])
	data := e.dataMap[""].(map[string]any)
	assert.Empty(t, data)
}

func TestEntry_SetData_JSONArray(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	jsonData := []byte(`[1, 2, 3]`)
	e.setData(jsonData, nil)

	assert.Nil(t, e.err)
	assert.NotNil(t, e.dataMap[""])
	data := e.dataMap[""].([]any)
	assert.Len(t, data, 3)
	assert.Equal(t, float64(1), data[0])
}

func TestEntry_SetData_ComplexJSON(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	jsonData := []byte(`{
		"users": [
			{"name": "alice", "age": 30},
			{"name": "bob", "age": 25}
		],
		"metadata": {
			"count": 2,
			"version": "1.0"
		}
	}`)
	e.setData(jsonData, nil)

	assert.Nil(t, e.err)
	assert.NotNil(t, e.dataMap[""])

	data := e.dataMap[""].(map[string]any)
	users := data["users"].([]any)
	assert.Len(t, users, 2)

	metadata := data["metadata"].(map[string]any)
	assert.Equal(t, float64(2), metadata["count"])
	assert.Equal(t, "1.0", metadata["version"])
}

func TestEntry_SetData_NilByteArray(t *testing.T) {
	e := &entry{
		dataMap:     make(map[string]any),
		err:         nil,
		projections: []store.Projection{},
	}

	var nilBytes []byte
	e.setData(nilBytes, nil)

	assert.Error(t, e.err)
	assert.Contains(t, e.err.Error(), "unexpected end of JSON input")
}

func TestEntry_SetData_OverwritesPreviousData(t *testing.T) {
	e := &entry{
		dataMap: map[string]any{
			"": map[string]any{"old": "data"},
		},
		err:         nil,
		projections: []store.Projection{},
	}

	jsonData := []byte(`{"new": "data"}`)
	e.setData(jsonData, nil)

	assert.Nil(t, e.err)
	data := e.dataMap[""].(map[string]any)
	assert.Equal(t, "data", data["new"])
	assert.NotContains(t, data, "old")
}

func TestEntry_SetData_ClearsErrorOnSuccess(t *testing.T) {
	// Verify that a successful API call clears any previous error,
	// even when there are no projections configured.
	e := &entry{
		dataMap:     make(map[string]any),
		err:         fmt.Errorf("previous error"),
		projections: []store.Projection{},
	}

	jsonData := []byte(`{"key": "value"}`)
	e.setData(jsonData, nil)

	// After a successful call, the error must be cleared so that
	// Get() returns the fresh data instead of the stale error.
	assert.Nil(t, e.err, "error should be cleared after successful data fetch with no projections")
	assert.NotNil(t, e.dataMap[""], "data should be set after successful fetch")
}

func TestEntry_SetData_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name        string
		data        any
		err         error
		projections []store.Projection
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid json with no projections",
			data:        []byte(`{"test": "data"}`),
			err:         nil,
			projections: []store.Projection{},
			wantErr:     false,
		},
		{
			name:        "invalid json",
			data:        []byte(`{invalid}`),
			err:         nil,
			projections: []store.Projection{},
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "error passed in",
			data:        nil,
			err:         fmt.Errorf("test error"),
			projections: []store.Projection{},
			wantErr:     true,
			errContains: "test error",
		},
		{
			name:        "non-byte data",
			data:        "string data",
			err:         nil,
			projections: []store.Projection{},
			wantErr:     true,
			errContains: "not a byte array",
		},
		{
			name: "projection error",
			data: []byte(`{"key": "value"}`),
			err:  nil,
			projections: []store.Projection{
				{
					Name: "test",
					JP: &mockJMESPathQuery{
						result: nil,
						err:    fmt.Errorf("projection failed"),
					},
				},
			},
			wantErr:     true,
			errContains: "projection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &entry{
				dataMap:     make(map[string]any),
				err:         nil,
				projections: tt.projections,
			}

			e.setData(tt.data, tt.err)

			if tt.wantErr {
				assert.Error(t, e.err)
				if tt.errContains != "" {
					assert.Contains(t, e.err.Error(), tt.errContains)
				}
			} else {
				assert.Nil(t, e.err)
			}
		})
	}
}
