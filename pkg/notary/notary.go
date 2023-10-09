package notary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/logging"
	_ "github.com/notaryproject/notation-core-go/signature/cose"
	_ "github.com/notaryproject/notation-core-go/signature/jws"
	"github.com/notaryproject/notation-go"
	notationlog "github.com/notaryproject/notation-go/log"
	"github.com/notaryproject/notation-go/verifier"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.uber.org/multierr"
)

var (
	maxReferrersCount = 50
	maxPayloadSize    = 10 * 1000 * 1000 // 10 MB
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

	v.log.V(4).Info("creating notation repo", "reference", opts.ImageRef)
	parsedRef, err := parseReferenceCrane(ctx, opts.ImageRef, opts.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image reference: %s", opts.ImageRef)
	}
	v.log.V(4).Info("created parsedRef", "reference", opts.ImageRef)

	ref := parsedRef.Ref.Name()
	remoteVerifyOptions := notation.VerifyOptions{
		ArtifactReference:    ref,
		MaxSignatureAttempts: 10,
	}

	targetDesc, outcomes, err := notation.Verify(notationlog.WithLogger(ctx, NotaryLoggerAdapter(v.log.WithName("Notary Verifier Debug"))), notationVerifier, parsedRef.Repo, remoteVerifyOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to verify %s", ref)
	}

	if err := v.verifyOutcomes(outcomes); err != nil {
		return nil, err
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

		v.log.V(2).Info("content", "type", contentType, "data", content)
	}

	return multierr.Combine(errs...)
}

func (v *notaryVerifier) FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	v.log.V(2).Info("fetching attestations", "reference", opts.ImageRef, "opts", opts)

	ref, err := name.ParseReference(opts.ImageRef)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image reference: %s", opts.ImageRef)
	}
	remoteOpts, err := opts.Client.BuildGCRRemoteOption(ctx)
	if err != nil {
		return nil, err
	}

	v.log.V(4).Info("client setup done", "repo", ref)

	repoDesc, err := gcrremote.Head(ref, remoteOpts...)
	if err != nil {
		return nil, err
	}
	v.log.V(4).Info("fetched repository", "repoDesc", repoDesc)

	referrers, err := gcrremote.Referrers(ref.Context().Digest(repoDesc.Digest.String()), remoteOpts...)
	if err != nil {
		return nil, err
	}

	referrersDescs, err := referrers.IndexManifest()
	if err != nil {
		return nil, err
	}

	// This check ensures that the manifest does not have an abnormal amount of referrers attached to it to protect against compromised images
	if len(referrersDescs.Manifests) > maxReferrersCount {
		return nil, fmt.Errorf("failed to fetch referrers: to many referrers found, max limit is %d", maxReferrersCount)
	}

	v.log.V(4).Info("fetched referrers", "referrers", referrersDescs)

	var statements []map[string]interface{}

	for _, referrer := range referrersDescs.Manifests {
		match, _, err := matchArtifactType(referrer, opts.Type)
		if err != nil {
			return nil, err
		}

		if !match {
			v.log.V(6).Info("type doesn't match, continue", "expected", opts.Type, "received", referrer.ArtifactType)
			continue
		}

		targetDesc, err := verifyAttestators(ctx, v, ref, opts, referrer)
		if err != nil {
			msg := err.Error()
			v.log.V(4).Info(msg, "failed to verify referrer %s", targetDesc.Digest.String())
			return nil, err
		}

		v.log.V(4).Info("extracting statements", "desc", referrer, "repo", ref)
		statements, err = extractStatements(ctx, ref, referrer, remoteOpts)
		if err != nil {
			msg := err.Error()
			v.log.V(4).Info("failed to extract statements %s", "err", msg)
			return nil, err
		}

		v.log.V(4).Info("verified attestators", "digest", targetDesc.Digest.String())

		if len(statements) == 0 {
			return nil, fmt.Errorf("failed to fetch attestations")
		}
		v.log.V(6).Info("sending response")
		return &images.Response{Digest: repoDesc.Digest.String(), Statements: statements}, nil
	}

	return nil, fmt.Errorf("failed to fetch attestations %s", err)
}

func verifyAttestators(ctx context.Context, v *notaryVerifier, ref name.Reference, opts images.Options, desc v1.Descriptor) (ocispec.Descriptor, error) {
	v.log.V(2).Info("verifying attestations", "reference", opts.ImageRef, "opts", opts)
	if opts.Cert == "" && opts.CertChain == "" {
		// skips the checks when no attestor is provided
		v1Desc := ocispec.Descriptor{
			MediaType:   string(desc.MediaType),
			Size:        desc.Size,
			Digest:      digest.Digest(desc.Digest.String()),
			URLs:        desc.URLs,
			Annotations: desc.Annotations,
			Data:        desc.Data,
		}
		return v1Desc, nil
	}
	certsPEM := combineCerts(opts)
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader([]byte(certsPEM)))
	if err != nil {
		v.log.V(4).Info("failed to parse certificates", "err", err)
		return ocispec.Descriptor{}, errors.Wrapf(err, "failed to parse certificates")
	}

	v.log.V(4).Info("parsed certificates")
	trustStore := NewTrustStore("kyverno", certs)
	policyDoc := v.buildPolicy()
	notationVerifier, err := verifier.New(policyDoc, trustStore, nil)
	if err != nil {
		v.log.V(4).Info("failed to created verifier", "err", err)
		return ocispec.Descriptor{}, errors.Wrapf(err, "failed to created verifier")
	}

	v.log.V(4).Info("created verifier")
	reference := ref.Context().RegistryStr() + "/" + ref.Context().RepositoryStr() + "@" + desc.Digest.String()
	parsedRef, err := parseReferenceCrane(ctx, reference, opts.Client)
	if err != nil {
		return ocispec.Descriptor{}, errors.Wrapf(err, "failed to parse image reference: %s", opts.ImageRef)
	}
	v.log.V(4).Info("created notation repo", "reference", opts.ImageRef)

	remoteVerifyOptions := notation.VerifyOptions{
		ArtifactReference:    reference,
		MaxSignatureAttempts: 10,
	}

	v.log.V(4).Info("verification started")
	targetDesc, outcomes, err := notation.Verify(context.TODO(), notationVerifier, parsedRef.Repo, remoteVerifyOptions)
	if err != nil {
		v.log.V(4).Info("failed to vefify attestator", "remoteVerifyOptions", remoteVerifyOptions, "repo", parsedRef.Repo)
		return targetDesc, err
	}
	if err := v.verifyOutcomes(outcomes); err != nil {
		return targetDesc, err
	}

	if targetDesc.Digest.String() != desc.Digest.String() {
		v.log.V(4).Info("digest mismatch", "expected", desc.Digest.String(), "found", targetDesc.Digest.String())
		return targetDesc, errors.Errorf("digest mismatch")
	}
	v.log.V(2).Info("attestator verified", "desc", targetDesc.Digest.String())

	return targetDesc, nil
}

func extractStatements(ctx context.Context, repoRef name.Reference, desc v1.Descriptor, remoteOpts []gcrremote.Option) ([]map[string]interface{}, error) {
	statements := make([]map[string]interface{}, 0)
	data, err := extractStatement(ctx, repoRef, desc, remoteOpts)
	if err != nil {
		return nil, err
	}
	statements = append(statements, data)

	if len(statements) == 0 {
		return nil, fmt.Errorf("no statements found")
	}
	return statements, nil
}

func extractStatement(ctx context.Context, repoRef name.Reference, desc v1.Descriptor, remoteOpts []gcrremote.Option) (map[string]interface{}, error) {
	refStr := repoRef.Context().RegistryStr() + "/" + repoRef.Context().RepositoryStr() + "@" + desc.Digest.String()
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image reference: %s", refStr)
	}

	remoteDesc, err := gcrremote.Get(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("error in fetching manifest: %w", err)
	}
	manifestBytes, err := remoteDesc.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("error in fetching statement: %w", err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, err
	}

	if len(manifest.Layers) == 0 {
		return nil, fmt.Errorf("no predicate found: %+v", manifest)
	}
	if len(manifest.Layers) > 1 {
		return nil, fmt.Errorf("multiple layers in predicate not supported: %+v", manifest)
	}
	predicateDesc := manifest.Layers[0]

	// This check ensures that the size of a layer isn't abnormally large to avoid malicious payloads
	if predicateDesc.Size > int64(maxPayloadSize) {
		return nil, fmt.Errorf("payload size is too large, max size is %d: %+v", maxPayloadSize, predicateDesc)
	}

	layer, err := gcrremote.Layer(ref.Context().Digest(predicateDesc.Digest.String()), remoteOpts...)
	if err != nil {
		return nil, err
	}
	ioPredicate, err := layer.Uncompressed()
	if err != nil {
		return nil, err
	}
	predicateBytes := new(bytes.Buffer)
	_, err = predicateBytes.ReadFrom(ioPredicate)
	if err != nil {
		return nil, err
	}

	predicate := make(map[string]interface{})
	if err := json.Unmarshal(predicateBytes.Bytes(), &predicate); err != nil {
		return nil, err
	}
	data := make(map[string]interface{})
	if err := json.Unmarshal(manifestBytes, &data); err != nil {
		return nil, err
	}

	if data["type"] == nil {
		data["type"] = desc.ArtifactType
	}
	if data["predicate"] == nil {
		data["predicate"] = predicate
	}
	return data, nil
}

func matchArtifactType(ref v1.Descriptor, expectedArtifactType string) (bool, string, error) {
	if expectedArtifactType != "" {
		if ref.ArtifactType == expectedArtifactType {
			return true, ref.ArtifactType, nil
		}
	}
	return false, "", nil
}
