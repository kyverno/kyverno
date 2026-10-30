package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitMetrics_HealthEndpoints(t *testing.T) {
	tests := []struct {
		name                 string
		disableMetricsExport bool
		otelProvider         string
		expectMetrics        bool
		expectProbes         bool
	}{
		{
			name:                 "prometheus with metrics enabled",
			disableMetricsExport: false,
			otelProvider:         "prometheus",
			expectMetrics:        true,
			expectProbes:         true,
		},
		{
			name:                 "prometheus with metrics disabled",
			disableMetricsExport: true,
			otelProvider:         "prometheus",
			expectMetrics:        false,
			expectProbes:         true,
		},
		{
			name:                 "grpc provider",
			disableMetricsExport: false,
			otelProvider:         "grpc",
			expectMetrics:        false,
			expectProbes:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultMetricsConfiguration()
			_, _, mux, _, err := InitMetrics(
				context.Background(),
				tt.disableMetricsExport,
				tt.otelProvider,
				8000,
				"collector",
				cfg,
				"",
				nil,
				nil,
				nil,
				"",
				"",
				"",
				logr.Discard(),
			)
			require.NoError(t, err)

			if !tt.expectProbes {
				assert.Nil(t, mux)
				return
			}

			require.NotNil(t, mux)

			// Probe paths must return 200 OK
			paths := []string{
				config.LivenessServicePath,
				config.ReadinessServicePath,
				"/healthz",
			}
			for _, path := range paths {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				mux.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusOK, rec.Code, "path %s should return 200 OK", path)
			}

			// Metrics endpoint
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, config.MetricsPath, nil)
			mux.ServeHTTP(rec, req)
			if tt.expectMetrics {
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Equal(t, http.StatusNotFound, rec.Code)
			}
		})
	}
}
