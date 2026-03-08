package notary

import (
	"context"
	"crypto/x509"

	"github.com/notaryproject/notation-go/verifier/truststore"
	"github.com/pkg/errors"
)

type simpleTrustStore struct {
	name      string
	storeType truststore.Type
	certs     []*x509.Certificate
}

func NewTrustStore(name string, certs []*x509.Certificate) truststore.X509TrustStore {
	return &simpleTrustStore{
		name:      name,
		storeType: truststore.TypeCA,
		certs:     certs,
	}
}

func (ts *simpleTrustStore) GetCertificates(ctx context.Context, storeType truststore.Type, name string) ([]*x509.Certificate, error) {
	if storeType != ts.storeType {
		return nil, errors.Errorf("invalid truststore type")
	}

	if name != ts.name {
		return nil, errors.Errorf("invalid truststore name")
	}

	return ts.certs, nil
}
