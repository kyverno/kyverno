package main

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
)

type mockCertValidator struct {
	valid bool
	err   error
}

func (m *mockCertValidator) ValidateCert(context.Context) (bool, error) {
	return m.valid, m.err
}

func TestProbesIsReady(t *testing.T) {
	tests := []struct {
		name     string
		valid    bool
		err      error
		expected bool
	}{
		{
			name:     "returns true when certs are valid",
			valid:    true,
			err:      nil,
			expected: true,
		},
		{
			name:     "returns false when certs are invalid",
			valid:    false,
			err:      nil,
			expected: false,
		},
		{
			name:     "returns false when validation errors",
			valid:    false,
			err:      errors.New("secret not found"),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := probes{logger: logr.Discard(), certValidator: &mockCertValidator{valid: tt.valid, err: tt.err}}
			if got := p.IsReady(context.Background()); got != tt.expected {
				t.Errorf("IsReady() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProbesIsLive(t *testing.T) {
	p := probes{logger: logr.Discard(), certValidator: &mockCertValidator{valid: false, err: errors.New("broken")}}
	if !p.IsLive(context.Background()) {
		t.Error("IsLive() should always return true")
	}
}
