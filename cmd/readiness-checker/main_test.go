package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHTTPClientVerifiesCertificatesByDefault(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := newHTTPClient(false).Get(server.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected TLS verification failure for self-signed certificate")
	}
}

func TestNewHTTPClientAllowsInsecureSkipVerifyWhenExplicitlyEnabled(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := newHTTPClient(true).Get(server.URL)
	if err != nil {
		t.Fatalf("expected HTTPS request to succeed with --insecure-skip-verify, got error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
