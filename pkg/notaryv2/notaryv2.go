package notaryv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/logging"
	_ "github.com/notaryproject/notation-core-go/signature/cose"
	_ "github.com/notaryproject/notation-core-go/signature/jws"
	"github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-go/verifier"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	"github.com/pkg/errors"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/config"
	"github.com/regclient/regclient/types"
	"github.com/regclient/regclient/types/ref"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.uber.org/multierr"
)

func NewVerifier() images.ImageVerifier {
	return &notaryV2Verifier{
		log: logging.WithName("NotaryV2"),
	}
}

type notaryV2Verifier struct {
	log logr.Logger
}

func (v *notaryV2Verifier) VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
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
	remoteVerifyOptions := notation.RemoteVerifyOptions{
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

func (v *notaryV2Verifier) buildPolicy() *trustpolicy.Document {
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

func (v *notaryV2Verifier) verifyOutcomes(outcomes []*notation.VerificationOutcome) error {
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

func (v *notaryV2Verifier) FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	v.log.V(3).Info("fetching attestations", "reference", opts.ImageRef)

	rcRepoReference, err := ref.New(opts.ImageRef)
	if err != nil {
		panic(err)
	}

	rcConfigHost := config.Host{
		Name:     rcRepoReference.Registry,
		Hostname: rcRepoReference.Registry,
	}

	rcClient := regclient.New(regclient.WithConfigHost(rcConfigHost))

	rcRepoReferrers, err := rcClient.ReferrerList(ctx, rcRepoReference)
	if err != nil {
		msg := err.Error()
		v.log.V(3).Info("failed to fetch attestations", "error", msg)
		if strings.Contains(msg, "MANIFEST_UNKNOWN: manifest unknown") {
			return nil, fmt.Errorf("not found")
		}

		return nil, err
	}
	rcReferrersDescs := rcRepoReferrers.Descriptors
	statements := make([]map[string]interface{}, 0)

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

	for _, referrer := range rcReferrersDescs {
		match, predicateType, err := matchArtifactType(referrer, opts.PredicateType)
		if err != nil {
			return nil, err
		}

		if !match {
			v.log.V(3).V(4).Info("predicateType doesn't match, continue", "expected", opts.PredicateType, "received", predicateType)
			continue
		}

		reference := rcRepoReferrers.Subject.Registry + "/" + rcRepoReference.Repository + "@" + referrer.Digest.String()
		repo, parsedRef, err := parseReference(ctx, reference, opts.RegistryClient)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse image reference: %s", opts.ImageRef)
		}

		refer := parsedRef.String()
		remoteVerifyOptions := notation.RemoteVerifyOptions{
			ArtifactReference:    refer,
			MaxSignatureAttempts: 10,
		}

		targetDesc, outcomes, err := notation.Verify(context.TODO(), notationVerifier, repo, remoteVerifyOptions)

		if err != nil {
			msg := err.Error()
			v.log.V(3).V(4).Info(msg, "failed to verify %s", refer)
			continue
		}
		if err := v.verifyOutcomes(outcomes); err != nil {
			msg := err.Error()
			v.log.V(3).V(4).Info(msg)
			continue
		}
		if targetDesc.Digest != referrer.Digest {
			v.log.V(3).V(4).Info("digest mismatch")
			continue
		}

		rcRefer, err := ref.New(rcRepoReference.Registry + "/" + rcRepoReference.Repository + "@" + targetDesc.Digest.String())
		if err != nil {
			msg := err.Error()
			v.log.V(3).Info(msg, "failed to create statements %s", targetDesc.Digest.String())
			continue
		}
		referrerManifest, err := rcClient.ManifestGet(ctx, rcRefer)
		if err != nil {
			msg := err.Error()
			v.log.V(3).Info(msg, "failed to create statements %s", rcRefer)
			continue
		}

		refManifestBody, err := referrerManifest.RawBody()
		if err != nil {
			msg := err.Error()
			v.log.V(3).Info(msg, "failed to create statements %s", rcRefer)
			continue
		}
		data := make(map[string]interface{})
		if err := json.Unmarshal(refManifestBody, &data); err != nil {
			msg := err.Error()
			v.log.V(3).Info(msg, "failed to create statements %s", rcRefer)
			continue
		}
		statements = append(statements, data)

		v.log.V(3).V(3).Info("verified images", "digest", len(targetDesc.Digest))
	}

	return &images.Response{Digest: rcRepoReference.Digest, Statements: statements}, nil
}

func matchArtifactType(ref types.Descriptor, expectedArtifactType string) (bool, string, error) {
	if expectedArtifactType != "" {
		if ref.ArtifactType == expectedArtifactType {
			return true, ref.ArtifactType, nil
		}
	}
	return false, "", nil
}
