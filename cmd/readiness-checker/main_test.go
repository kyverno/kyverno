package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

func boolPtr(b bool) *bool {
	return &b
}

func newEndpointSlice(name, namespace, svcName string, ready *bool) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Service",
					Name: svcName,
				},
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Conditions: discoveryv1.EndpointConditions{
					Ready: ready,
				},
			},
		},
	}
}

func TestAttemptCheckEndpoints_ListPath(t *testing.T) {
	tests := []struct {
		name    string
		ready   *bool
		wantErr error
	}{
		{name: "ready", ready: boolPtr(true), wantErr: nil},
		{name: "not-ready", ready: boolPtr(false), wantErr: errNoReadyEndpoints},
		{name: "nil-ready", ready: nil, wantErr: errNoReadyEndpoints},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice := newEndpointSlice("svc-abc123", "default", "svc", tt.ready)
			clientset := fake.NewClientset(slice)

			err := attemptCheckEndpoints(context.Background(), clientset, "svc", "default", nil)
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAttemptCheckEndpoints_GetPath(t *testing.T) {
	tests := []struct {
		name    string
		ready   *bool
		wantErr error
	}{
		{name: "ready", ready: boolPtr(true), wantErr: nil},
		{name: "not-ready", ready: boolPtr(false), wantErr: errNoReadyEndpoints},
		{name: "nil-ready", ready: nil, wantErr: errNoReadyEndpoints},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice := newEndpointSlice("svc-abc123", "default", "svc", tt.ready)
			clientset := fake.NewClientset(slice)

			err := attemptCheckEndpoints(context.Background(), clientset, "svc", "default", []string{"svc-abc123"})
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
