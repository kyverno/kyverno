package main

import (
	"context"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

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
