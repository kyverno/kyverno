package notary

import (
	"bytes"
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/logging"
	_ "github.com/notaryproject/notation-core-go/signature/cose"
	_ "github.com/notaryproject/notation-core-go/signature/jws"
	"github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-go/verifier"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.uber.org/multierr"
)

func NewVerifier() images.ImageVerifier {
	return &notaryVerifier{
		log: logging.WithName("Notary"),
	}
}

type notaryVerifier struct {
	log logr.Logger
}

func (v *notaryVerifier) VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	v.log.V(2).Info("verifying image", "reference", opts.ImageRef)

	certsPEM := combineCerts(opts)
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader([]byte(certsPEM)))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse certificates")
	}

	trustStore := NewTrustStore("kyverno", certs)
	policyDoc := v.buildPolicy()
	notationVerifier, err := verifier.New(policyDoc, trustStore, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to created verifier")
	}

	repo, parsedRef, err := parseReference(ctx, opts.ImageRef, opts.RegistryClient)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image reference: %s", opts.ImageRef)
	}

	digest, err := parsedRef.Digest()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch digest")
	}

	ref := parsedRef.String()
	remoteVerifyOptions := notation.VerifyOptions{
		ArtifactReference:    ref,
		MaxSignatureAttempts: 10,
	}

	targetDesc, outcomes, err := notation.Verify(context.TODO(), notationVerifier, repo, remoteVerifyOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to verify %s", ref)
	}

	if err := v.verifyOutcomes(outcomes); err != nil {
		return nil, err
	}

	if targetDesc.Digest != digest {
		return nil, errors.Errorf("digest mismatch")
	}

	v.log.V(2).Info("verified image", "type", targetDesc.MediaType, "digest", targetDesc.Digest, "size", targetDesc.Size)

	resp := &images.Response{
		Digest:     targetDesc.Digest.String(),
		Statements: nil,
	}

	return resp, nil
}

func combineCerts(opts images.Options) string {
	certs := opts.Cert
	if opts.CertChain != "" {
		if certs != "" {
			certs = certs + "\n"
		}

		certs = certs + opts.CertChain
	}

	return certs
}

func (v *notaryVerifier) buildPolicy() *trustpolicy.Document {
	return &trustpolicy.Document{
		Version: "1.0",
		TrustPolicies: []trustpolicy.TrustPolicy{
			{
				Name:                  "kyverno",
				RegistryScopes:        []string{"*"},
				SignatureVerification: trustpolicy.SignatureVerification{VerificationLevel: trustpolicy.LevelStrict.Name},
				TrustStores:           []string{"ca:kyverno"},
				TrustedIdentities:     []string{"*"},
			},
		},
	}
}

func (v *notaryVerifier) verifyOutcomes(outcomes []*notation.VerificationOutcome) error {
	var errs []error
	for _, outcome := range outcomes {
		if outcome.Error != nil {
			errs = append(errs, outcome.Error)
			continue
		}

		content := outcome.EnvelopeContent.Payload.Content
		contentType := outcome.EnvelopeContent.Payload.ContentType

		v.log.Info("content", "type", contentType, "data", content)
	}

	return multierr.Combine(errs...)
}

func (v *notaryVerifier) FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	return nil, errors.Errorf("not implemented")
}
