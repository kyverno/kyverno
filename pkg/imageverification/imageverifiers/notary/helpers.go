package notary

import (
	"bytes"
	"context"
	"crypto/x509"

	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/notaryproject/notation-go"
	notationregistry "github.com/notaryproject/notation-go/registry"
	"github.com/notaryproject/notation-go/verifier"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	"github.com/notaryproject/notation-go/verifier/truststore"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.uber.org/multierr"
)

type simpleTrustStore struct {
	name     string
	cacerts  []*x509.Certificate
	tsacerts []*x509.Certificate
}

func NewTrustStore(name string, certs []*x509.Certificate, tsaCerts []*x509.Certificate) truststore.X509TrustStore {
	return &simpleTrustStore{
		name:     name,
		cacerts:  certs,
		tsacerts: tsaCerts,
	}
}

func (ts *simpleTrustStore) GetCertificates(ctx context.Context, storeType truststore.Type, name string) ([]*x509.Certificate, error) {
	if name != ts.name {
		return nil, errors.New("truststore not found")
	}
	switch storeType {
	case truststore.TypeCA:
		return ts.cacerts, nil
	case truststore.TypeTSA:
		return ts.tsacerts, nil
	}
	return nil, errors.New("entry not found in trust store")
}

func buildTrustPolicy(tsa []*x509.Certificate) *trustpolicy.Document {
	truststores := []string{"ca:kyverno"}
	if len(tsa) != 0 {
		truststores = append(truststores, "tsa:kyverno")
	}
	return &trustpolicy.Document{
		Version: "1.0",
		TrustPolicies: []trustpolicy.TrustPolicy{
			{
				Name:                  "kyverno",
				RegistryScopes:        []string{"*"},
				SignatureVerification: trustpolicy.SignatureVerification{VerificationLevel: trustpolicy.LevelStrict.Name},
				TrustStores:           truststores,
				TrustedIdentities:     []string{"*"},
			},
		},
	}
}

func checkVerificationOutcomes(outcomes []*notation.VerificationOutcome) error {
	var errs []error
	for _, outcome := range outcomes {
		if outcome.Error != nil {
			errs = append(errs, outcome.Error)
			continue
		}
	}
	return multierr.Combine(errs...)
}

type verificationInfo struct {
	Verifier notation.Verifier
	Repo     notationregistry.Repository
}

func getVerificationInfo(image *imagedataloader.ImageData, certsData, tsaCertsData string) (*verificationInfo, error) {
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader([]byte(certsData)))
	if err != nil {
		return nil, err
	}
	tsacerts, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader([]byte(tsaCertsData)))
	if err != nil {
		return nil, err
	}
	notationVerifier, err := verifier.New(buildTrustPolicy(tsacerts), NewTrustStore("kyverno", certs, tsacerts), nil)
	if err != nil {
		return nil, err
	}
	return &verificationInfo{
		Verifier: notationVerifier,
		Repo:     NewRepository(image),
	}, nil
}
