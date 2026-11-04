package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScopeKeyReference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		keyRef  string
		want    string
		wantErr bool
	}{
		{name: "inline key", keyRef: "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----", want: "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----"},
		{name: "kms uri", keyRef: "awskms:///arn:aws:kms:us-east-1:1:key/2", want: "awskms:///arn:aws:kms:us-east-1:1:key/2"},
		{name: "empty", keyRef: "", want: ""},
		{name: "same namespace", keyRef: "k8s://tenant-a/cosign", want: "k8s://tenant-a/cosign"},
		{name: "surrounding space", keyRef: "  k8s://tenant-a/cosign\n", want: "k8s://tenant-a/cosign"},
		{name: "foreign namespace", keyRef: "k8s://kyverno/stolen-key", wantErr: true},
		{name: "foreign namespace mixed case scheme", keyRef: "K8S://kyverno/stolen-key", wantErr: true},
		{name: "missing namespace", keyRef: "k8s://stolen-key", wantErr: true},
		{name: "empty namespace", keyRef: "k8s:///stolen-key", wantErr: true},
		{name: "extra segment", keyRef: "k8s://tenant-a/cosign/extra", wantErr: true},
		{name: "empty name", keyRef: "k8s://tenant-a/", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ScopeKeyReference(tt.keyRef, "tenant-a")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScopeKeySecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		namespace string
		secret    string
		want      string
		wantErr   bool
	}{
		{name: "same namespace", namespace: "tenant-a", secret: "cosign", want: "k8s://tenant-a/cosign"},
		{name: "empty namespace defaults to policy namespace", secret: "cosign", want: "k8s://tenant-a/cosign"},
		{name: "foreign namespace", namespace: "kyverno", secret: "stolen-key", wantErr: true},
		{name: "empty name", namespace: "tenant-a", wantErr: true},
		{name: "name with slash", namespace: "tenant-a", secret: "kyverno/stolen-key", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ScopeKeySecret(tt.namespace, tt.secret, "tenant-a")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
