package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestWebhookAuthenticator_Disabled(t *testing.T) {
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	kubeClient := fake.NewSimpleClientset()
	authenticator := NewWebhookAuthenticator(innerHandler, kubeClient, false)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	authenticator.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestWebhookAuthenticator_Enabled_MissingHeader(t *testing.T) {
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	kubeClient := fake.NewSimpleClientset()
	authenticator := NewWebhookAuthenticator(innerHandler, kubeClient, true)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	authenticator.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestWebhookAuthenticator_Enabled_InvalidHeaderFormat(t *testing.T) {
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	kubeClient := fake.NewSimpleClientset()
	authenticator := NewWebhookAuthenticator(innerHandler, kubeClient, true)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "InvalidToken")
	rr := httptest.NewRecorder()

	authenticator.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// Since generating a complete valid JWT with a corresponding fake JWKS served by fake KubeClient
// is complex for a unit test, we just test that an invalid token fails properly with 401.
func TestWebhookAuthenticator_Enabled_InvalidToken(t *testing.T) {
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	kubeClient := fake.NewSimpleClientset()
	authenticator := NewWebhookAuthenticator(innerHandler, kubeClient, true)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-jwt-token")
	rr := httptest.NewRecorder()

	authenticator.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}
