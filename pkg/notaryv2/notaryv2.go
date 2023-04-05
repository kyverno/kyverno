package notaryv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/logging"
	_ "github.com/notaryproject/notation-core-go/signature/cose"
	_ "github.com/notaryproject/notation-core-go/signature/jws"
	"github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-go/verifier"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/regclient/regclient/types/ref"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.uber.org/multierr"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	orasReg "oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
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

	repo, err := remote.NewRepository(opts.ImageRef)
	if err != nil {
		return nil, err
	}

	repoDesc, err := oras.Resolve(ctx, repo, repo.Reference.Reference, oras.DefaultResolveOptions)
	if err != nil {
		return nil, err
	}

	referrersDescs, err := fetchReferrers(ctx, repo, repoDesc)
	if err != nil {
		return nil, err
	}

	var statements []map[string]interface{}

	for _, referrer := range referrersDescs {
		match, predicateType, err := matchArtifactType(referrer, opts.PredicateType)
		if err != nil {
			return nil, err
		}

		if !match {
			v.log.V(3).Info("predicateType doesn't match, continue", "expected", opts.PredicateType, "received", predicateType)
			continue
		}

		targetDesc, err := verifyAttestators(ctx, v, opts, referrer)
		if err != nil {
			msg := err.Error()
			v.log.V(3).Info(msg, "failed to verify referrer %s", targetDesc.Digest.String())
			continue
		}

		statements, err = extractStatements(ctx, repo, targetDesc)
		if err != nil {
			msg := err.Error()
			v.log.V(3).Info(msg, "failed to extract statements %s", targetDesc.Digest.String())
			continue
		}

		v.log.V(3).Info("verified images", "digest", len(targetDesc.Digest))
		break
	}

	return &images.Response{Digest: repoDesc.Digest.String(), Statements: statements}, nil
}

func verifyAttestators(ctx context.Context, v *notaryV2Verifier, opts images.Options, desc ocispec.Descriptor) (ocispec.Descriptor, error) {
	rcRepoReference, err := ref.New(opts.ImageRef)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if opts.Cert == "" && opts.CertChain == "" {
		// skips the checks when no attestor is provided
		return desc, nil
	}
	certsPEM := combineCerts(opts)
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader([]byte(certsPEM)))
	if err != nil {
		return ocispec.Descriptor{}, errors.Wrapf(err, "failed to parse certificates")
	}

	trustStore := NewTrustStore("kyverno", certs)
	policyDoc := v.buildPolicy()
	notationVerifier, err := verifier.New(policyDoc, trustStore, nil)
	if err != nil {
		return ocispec.Descriptor{}, errors.Wrapf(err, "failed to created verifier")
	}

	reference := rcRepoReference.Registry + "/" + rcRepoReference.Repository + "@" + desc.Digest.String()
	repo, parsedRef, err := parseReference(ctx, reference, opts.RegistryClient)
	if err != nil {
		return ocispec.Descriptor{}, errors.Wrapf(err, "failed to parse image reference: %s", opts.ImageRef)
	}

	refer := parsedRef.String()
	remoteVerifyOptions := notation.RemoteVerifyOptions{
		ArtifactReference:    refer,
		MaxSignatureAttempts: 10,
	}

	targetDesc, outcomes, err := notation.Verify(context.TODO(), notationVerifier, repo, remoteVerifyOptions)
	if err != nil {
		return targetDesc, err
	}
	if err := v.verifyOutcomes(outcomes); err != nil {
		return targetDesc, err
	}
	if targetDesc.Digest != desc.Digest {
		return targetDesc, errors.Errorf("digest mismatch")
	}

	return targetDesc, nil
}

func extractStatements(ctx context.Context, repo *remote.Repository, targetDesc ocispec.Descriptor) ([]map[string]interface{}, error) {
	statements := make([]map[string]interface{}, 0)
	data, err := extractStatement(ctx, repo, targetDesc)
	if err != nil {
		return nil, err
	}
	statements = append(statements, data)
	return statements, nil
}

func extractStatement(ctx context.Context, repo *remote.Repository, targetDesc ocispec.Descriptor) (map[string]interface{}, error) {
	repoDesc, artifactListIO, err := oras.Fetch(ctx, repo, repo.Reference.Reference, oras.DefaultFetchOptions)

	if err != nil {
		return nil, err
	}
	if repoDesc.Digest != targetDesc.Digest {
		return nil, errors.Errorf("Couldn't fetch statement")
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(artifactListIO)

	manifest := ocispec.Manifest{}
	if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
		return nil, fmt.Errorf("error decoding the payload: %w", err)
	}

	data := make(map[string]interface{})
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func matchArtifactType(ref ocispec.Descriptor, expectedArtifactType string) (bool, string, error) {
	if expectedArtifactType != "" {
		if ref.ArtifactType == expectedArtifactType {
			return true, ref.ArtifactType, nil
		}
	}
	return false, "", nil
}

func fetchReferrers(ctx context.Context, src content.ReadOnlyGraphStorage, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	var results []ocispec.Descriptor
	if repo, ok := src.(orasReg.ReferrerLister); ok {
		err := repo.Referrers(ctx, desc, "", func(referrers []ocispec.Descriptor) error {
			results = append(results, referrers...)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return results, nil
	}
	predecessors, err := src.Predecessors(ctx, desc)
	if err != nil {
		return nil, err
	}
	for _, node := range predecessors {
		switch node.MediaType {
		case ocispec.MediaTypeArtifactManifest, ocispec.MediaTypeImageManifest:
			results = append(results, node)
		}
	}
	return results, nil
}
