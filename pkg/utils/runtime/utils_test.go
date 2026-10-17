package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/require"
)

type certValidator struct {
	valid bool
	err   error
}

func (c certValidator) ValidateCert(context.Context) (bool, error) { return c.valid, c.err }

func TestReadinessAndLiveness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cert   certValidator
		checks []func() bool
		code   int
	}{
		{"existing caller", certValidator{valid: true}, nil, http.StatusOK},
		{"healthy informer", certValidator{valid: true}, []func() bool{func() bool { return true }}, http.StatusOK},
		{"unhealthy informer", certValidator{valid: true}, []func() bool{func() bool { return false }}, http.StatusInternalServerError},
		{"invalid certificate", certValidator{}, []func() bool{func() bool { return true }}, http.StatusInternalServerError},
		{"certificate error", certValidator{err: errors.New("unavailable")}, nil, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &runtime{logger: logr.Discard(), certValidator: tc.cert, readinessChecks: tc.checks}
			out := httptest.NewRecorder()
			handlers.Probe(r.IsReady)(out, httptest.NewRequest(http.MethodGet, "/health/readiness", nil))
			require.Equal(t, tc.code, out.Code)
			require.True(t, r.IsLive(t.Context()))
		})
	}
}
