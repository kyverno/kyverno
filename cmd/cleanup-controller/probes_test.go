package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/webhooks"
)

type testCertValidator func(context.Context) (bool, error)

func (f testCertValidator) ValidateCert(ctx context.Context) (bool, error) {
	return f(ctx)
}

type testContextKey struct{}

func TestProbesIsReady(t *testing.T) {
	validationErr := errors.New("validation failed")
	tests := []struct {
		name                string
		caSecretSynced      bool
		tlsSecretSynced     bool
		valid               bool
		validationErr       error
		want                bool
		wantCASecretChecks  int
		wantTLSSecretChecks int
		wantValidationCalls int
	}{
		{
			name:                "certificates are valid",
			caSecretSynced:      true,
			tlsSecretSynced:     true,
			valid:               true,
			want:                true,
			wantCASecretChecks:  1,
			wantTLSSecretChecks: 1,
			wantValidationCalls: 1,
		},
		{
			name:               "CA secret informer is unsynced",
			caSecretSynced:     false,
			tlsSecretSynced:    true,
			want:               false,
			wantCASecretChecks: 1,
		},
		{
			name:                "TLS secret informer is unsynced",
			caSecretSynced:      true,
			tlsSecretSynced:     false,
			want:                false,
			wantCASecretChecks:  1,
			wantTLSSecretChecks: 1,
		},
		{
			name:                "certificates are invalid",
			caSecretSynced:      true,
			tlsSecretSynced:     true,
			valid:               false,
			want:                false,
			wantCASecretChecks:  1,
			wantTLSSecretChecks: 1,
			wantValidationCalls: 1,
		},
		{
			name:                "certificate validation fails",
			caSecretSynced:      true,
			tlsSecretSynced:     true,
			validationErr:       validationErr,
			want:                false,
			wantCASecretChecks:  1,
			wantTLSSecretChecks: 1,
			wantValidationCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var caSecretChecks, tlsSecretChecks, validationCalls int
			ctx := context.WithValue(context.Background(), testContextKey{}, tt.name)
			var validationContext context.Context
			probe := probes{
				logger: logr.Discard(),
				certValidator: testCertValidator(func(ctx context.Context) (bool, error) {
					validationCalls++
					validationContext = ctx
					return tt.valid, tt.validationErr
				}),
				caSecretSynced: func() bool {
					caSecretChecks++
					return tt.caSecretSynced
				},
				tlsSecretSynced: func() bool {
					tlsSecretChecks++
					return tt.tlsSecretSynced
				},
			}

			if got := probe.IsReady(ctx); got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
			if caSecretChecks != tt.wantCASecretChecks {
				t.Errorf("CA secret sync checks = %d, want %d", caSecretChecks, tt.wantCASecretChecks)
			}
			if tlsSecretChecks != tt.wantTLSSecretChecks {
				t.Errorf("TLS secret sync checks = %d, want %d", tlsSecretChecks, tt.wantTLSSecretChecks)
			}
			if validationCalls != tt.wantValidationCalls {
				t.Errorf("validation calls = %d, want %d", validationCalls, tt.wantValidationCalls)
			}
			if validationCalls > 0 && validationContext != ctx {
				t.Error("ValidateCert() did not receive the request context")
			}
		})
	}
}

func TestProbesIsLive(t *testing.T) {
	if !(probes{}).IsLive(context.Background()) {
		t.Error("IsLive() = false, want true")
	}
}

func TestServerProbeStatus(t *testing.T) {
	tests := []struct {
		name           string
		certsAreValid  bool
		wantStatusCode int
	}{
		{name: "ready", certsAreValid: true, wantStatusCode: http.StatusOK},
		{name: "not ready", certsAreValid: false, wantStatusCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := probes{
				logger:          logr.Discard(),
				certValidator:   testCertValidator(func(context.Context) (bool, error) { return tt.certsAreValid, nil }),
				caSecretSynced:  func() bool { return true },
				tlsSecretSynced: func() bool { return true },
			}
			cleanupServer, ok := NewServer(
				func() ([]byte, []byte, error) { return nil, nil, nil },
				nil,
				nil,
				nil,
				webhooks.DebugModeOptions{},
				probe,
				nil,
			).(*server)
			if !ok {
				t.Fatal("NewServer() did not return a *server")
			}

			request := httptest.NewRequest(http.MethodGet, config.ReadinessServicePath, nil)
			response := httptest.NewRecorder()
			cleanupServer.server.Handler.ServeHTTP(response, request)
			if response.Code != tt.wantStatusCode {
				t.Errorf("readiness status code = %d, want %d", response.Code, tt.wantStatusCode)
			}
		})
	}
}
