package main

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

type mockCertValidator struct {
	valid bool
	err   error
}

func (m *mockCertValidator) ValidateCert(_ context.Context) (bool, error) {
	return m.valid, m.err
}

func TestProbes_IsLive(t *testing.T) {
	t.Parallel()
	// IsLive should always return true regardless of cert state.
	p := newProbes(&mockCertValidator{valid: false, err: errors.New("ignored")}, logr.Discard())
	assert.True(t, p.IsLive(context.Background()))
}

func TestProbes_IsReady_ValidCert(t *testing.T) {
	t.Parallel()
	// IsReady should return true when the cert validator reports the cert is valid.
	p := newProbes(&mockCertValidator{valid: true, err: nil}, logr.Discard())
	assert.True(t, p.IsReady(context.Background()))
}

func TestProbes_IsReady_InvalidCert(t *testing.T) {
	t.Parallel()
	// IsReady should return false when the cert validator reports the cert is invalid
	p := newProbes(&mockCertValidator{valid: false, err: nil}, logr.Discard())
	assert.False(t, p.IsReady(context.Background()))
}

func TestProbes_IsReady_ValidatorError(t *testing.T) {
	t.Parallel()
	// IsReady should return false and not panic when cert validation returns an error.
	p := newProbes(&mockCertValidator{valid: false, err: errors.New("secret not found")}, logr.Discard())
	assert.False(t, p.IsReady(context.Background()))
}
