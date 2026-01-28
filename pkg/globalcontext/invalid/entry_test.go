package invalid

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	testErr := errors.New("test error")
	
	e := New(testErr)
	
	assert.NotNil(t, e)
	assert.Equal(t, testErr, e.err)
}

func TestNew_WithNilError(t *testing.T) {
	e := New(nil)
	
	assert.NotNil(t, e)
	assert.Nil(t, e.err)
}

func TestEntry_Get_ReturnsWrappedError(t *testing.T) {
	testErr := errors.New("original error")
	e := New(testErr)
	
	result, err := e.Get()
	
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cached context entry")
	assert.Contains(t, err.Error(), "original error")
}

func TestEntry_Get_WithNilError(t *testing.T) {
	e := New(nil)
	
	result, err := e.Get()
	
	assert.Nil(t, result)
	// Even with nil error, the wrapper still creates an error message
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cached context entry")
}

func TestEntry_Get_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name        string
		inputErr    error
		expectNil   bool
		containsMsg string
	}{
		{
			name:        "with simple error",
			inputErr:    errors.New("simple error"),
			expectNil:   false,
			containsMsg: "simple error",
		},
		{
			name:        "with wrapped error",
			inputErr:    errors.New("wrapped: inner error"),
			expectNil:   false,
			containsMsg: "inner error",
		},
		{
			name:        "with empty error message",
			inputErr:    errors.New(""),
			expectNil:   false,
			containsMsg: "failed to create cached context entry",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.inputErr)
			
			result, err := e.Get()
			
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.containsMsg)
		})
	}
}

func TestEntry_Stop_DoesNotPanic(t *testing.T) {
	e := New(errors.New("test"))
	
	// Stop should not panic even though it's a no-op
	assert.NotPanics(t, func() {
		e.Stop()
	})
}

func TestEntry_Stop_CalledMultipleTimes(t *testing.T) {
	e := New(errors.New("test"))
	
	// Multiple calls should be safe
	assert.NotPanics(t, func() {
		e.Stop()
		e.Stop()
		e.Stop()
	})
}

func TestEntry_Stop_OnNilError(t *testing.T) {
	e := New(nil)
	
	assert.NotPanics(t, func() {
		e.Stop()
	})
}

func TestEntry_Get_AlwaysReturnsNilResult(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
	}{
		{"with error", errors.New("error")},
		{"with nil error", nil},
		{"with empty error", errors.New("")},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.inputErr)
			
			result, _ := e.Get()
			
			assert.Nil(t, result, "Get should always return nil result")
		})
	}
}

func TestEntry_Get_AlwaysReturnsError(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
	}{
		{"with error", errors.New("error")},
		{"with nil error", nil},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.inputErr)
			
			_, err := e.Get()
			
			assert.Error(t, err, "Get should always return an error")
		})
	}
}
