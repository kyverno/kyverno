package invalid

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntry(t *testing.T) {
	tests := []struct {
		name       string
		inputErr   error
		wantNil    bool
		wantErrMsg string
	}{
		{"simple error", errors.New("connection failed"), true, "failed to create cached context entry: connection failed"},
		{"wrapped error", errors.New("timeout"), true, "failed to create cached context entry: timeout"},
		{"empty message", errors.New(""), true, "failed to create cached context entry: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := New(tt.inputErr)

			val, err := entry.Get()
			assert.Nil(t, val)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestEntry_Stop(t *testing.T) {
	entry := New(errors.New("test error"))
	// Stop should not panic
	assert.NotPanics(t, func() {
		entry.Stop()
	})
}

func TestNew(t *testing.T) {
	err := errors.New("test")
	entry := New(err)
	assert.NotNil(t, entry)
}
