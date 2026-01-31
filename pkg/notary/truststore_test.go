package notary

import (
	"context"
	"crypto/x509"
	"testing"

	"github.com/notaryproject/notation-go/verifier/truststore"
)

func TestNewTrustStore(t *testing.T) {
	certs := []*x509.Certificate{{}, {}}
	ts := NewTrustStore("test-store", certs)

	if ts == nil {
		t.Fatal("NewTrustStore returned nil")
	}
}

func TestSimpleTrustStore_GetCertificates(t *testing.T) {
	certs := []*x509.Certificate{{}, {}}
	ts := NewTrustStore("my-store", certs)

	tests := []struct {
		name      string
		storeType truststore.Type
		storeName string
		wantErr   bool
		wantLen   int
	}{
		{"valid request", truststore.TypeCA, "my-store", false, 2},
		{"wrong store type", truststore.TypeSigningAuthority, "my-store", true, 0},
		{"wrong store name", truststore.TypeCA, "wrong-name", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ts.GetCertificates(context.Background(), tt.storeType, tt.storeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCertificates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("GetCertificates() len = %v, want %v", len(got), tt.wantLen)
			}
		})
	}
}

func TestSimpleTrustStore_EmptyCerts(t *testing.T) {
	ts := NewTrustStore("empty-store", nil)
	got, err := ts.GetCertificates(context.Background(), truststore.TypeCA, "empty-store")
	if err != nil {
		t.Errorf("GetCertificates() unexpected error = %v", err)
	}
	if got != nil && len(got) != 0 {
		t.Errorf("GetCertificates() expected nil or empty, got len = %v", len(got))
	}
}
