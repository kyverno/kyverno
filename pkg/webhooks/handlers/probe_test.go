package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type headerTrackingResponseWriter struct {
	http.ResponseWriter
	statusCodes []int
}

func (w *headerTrackingResponseWriter) WriteHeader(code int) {
	w.statusCodes = append(w.statusCodes, code)
	w.ResponseWriter.WriteHeader(code)
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name            string
		check           func(context.Context) bool
		wantFinalStatus int
		wantStatusCodes []int
	}{
		{
			name:            "nil check returns 200",
			check:           nil,
			wantFinalStatus: http.StatusOK,
			wantStatusCodes: []int{http.StatusOK},
		},
		{
			name: "check returning true returns 200",
			check: func(ctx context.Context) bool {
				return true
			},
			wantFinalStatus: http.StatusOK,
			wantStatusCodes: []int{http.StatusOK},
		},
		{
			name: "check returning false returns 500 without secondary 200 write",
			check: func(ctx context.Context) bool {
				return false
			},
			wantFinalStatus: http.StatusInternalServerError,
			wantStatusCodes: []int{http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Probe(tt.check)
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			tracker := &headerTrackingResponseWriter{
				ResponseWriter: rec,
			}

			handler.ServeHTTP(tracker, req)

			assert.Equal(t, tt.wantFinalStatus, rec.Code)
			assert.Equal(t, tt.wantStatusCodes, tracker.statusCodes)
		})
	}
}
