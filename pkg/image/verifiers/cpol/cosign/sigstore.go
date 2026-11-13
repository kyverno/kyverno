package cosign

import (
	"context"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/sigstoretuf"
	"github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/pkg/errors"
	sigs "github.com/sigstore/cosign/v3/pkg/signature"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/signature"
)

var attestationlimit = 50

type verificationResult struct {
	Bundle *verificationBundle
	Result *verify.VerificationResult
	Desc   *v1.Descriptor
}

type verificationBundle struct {
	ProtoBundle   *bundle.Bundle
	DSSE_Envelope *in_toto.Statement //nolint:staticcheck
}

func verifyBundleAndFetchAttestations(ctx context.Context, opts verifiers.Options) ([]*verificationResult, error) {
	nameOpts := opts.Client.NameOptions()
	ref, err := name.ParseReference(opts.ImageRef, nameOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image reference: %v", opts.ImageRef)
	}
	remoteOpts, _, err := opts.Client.Options(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create remote opts: %v", opts.ImageRef)
	}
	bundles, desc, missing, err := fetchBundles(ref, attestationlimit, opts.Type, remoteOpts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch bundles: %v", opts.ImageRef)
	}
	policy, err := buildPolicy(desc, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build policy: %v", opts.ImageRef)
	}
	verifyOpts := buildVerifyOptions(opts)
	trustedMaterial, err := getTrustedMaterial(ctx, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get trusted material: %v", opts.ImageRef)
	}
	results, err := verifyBundles(bundles, desc, trustedMaterial, policy, verifyOpts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get verify bundles: %v", opts.ImageRef)
	}
	// a referrer listed but gone may have been the signature, so it is only ignored when another bundle verified
	if len(results) == 0 && len(missing) > 0 {
		return nil, errors.Wrapf(stderrors.Join(missing...), "no bundle verified and some referrers were missing: %v", opts.ImageRef)
	}
	return results, nil
}

func verifyBundles(bundles []*verificationBundle, desc *v1.Descriptor, trustedMaterial root.TrustedMaterial, policy verify.PolicyBuilder, verifierOpts []verify.VerifierOption) ([]*verificationResult, error) {
	verifier, err := verify.NewSignedEntityVerifier(trustedMaterial, verifierOpts...)
	if err != nil {
		return nil, err
	}
	verificationResults := make([]*verificationResult, 0)
	for _, bundle := range bundles {
		result, err := verifier.Verify(bundle.ProtoBundle, policy)
		if err == nil {
			verificationResults = append(verificationResults, &verificationResult{Bundle: bundle, Result: result, Desc: desc})
		} else {
			logger.V(4).Info("failed to verify sigstore bundle", "err", err.Error(), "bundle", bundle)
		}
	}
	return verificationResults, nil
}

const (
	sigstoreBundleArtifactType = "application/vnd.dev.sigstore.bundle"
	ociEmptyArtifactType       = "application/vnd.oci.empty.v1+json"
)

// isSigstoreBundleType reports whether a media or artifact type is any version of the sigstore bundle type.
func isSigstoreBundleType(mediaType string) bool {
	return strings.HasPrefix(mediaType, sigstoreBundleArtifactType)
}

// isSigstoreBundleManifest reports whether the manifest artifact type or its first layer media type is a sigstore bundle.
func isSigstoreBundleManifest(manifest *v1.Manifest) bool {
	if isSigstoreBundleType(manifest.ArtifactType) {
		return true
	}
	return len(manifest.Layers) > 0 && isSigstoreBundleType(string(manifest.Layers[0].MediaType))
}

// isNotFound reports whether the registry answered that the requested manifest does not exist.
func isNotFound(err error) bool {
	var terr *transport.Error
	return errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound
}

// fetchBundles lists the referrers of ref and returns the sigstore bundles among them, filtered by predicateType when set,
// along with the descriptor of the image the bundles must cover. Referrers whose manifest is gone are skipped and
// returned in missing, so the caller can report them when no bundle verifies.
func fetchBundles(ref name.Reference, limit int, predicateType string, remoteOpts []remote.Option) ([]*verificationBundle, *v1.Descriptor, []error, error) {
	bundles := make([]*verificationBundle, 0)
	var missing []error
	desc, err := remote.Head(ref, remoteOpts...)
	if err != nil {
		return nil, nil, nil, err
	}
	referrers, err := remote.Referrers(ref.Context().Digest(desc.Digest.String()), remoteOpts...)
	if err != nil {
		return nil, nil, nil, err
	}
	referrersDescs, err := verifiers.ReferrersIndexManifest(referrers)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(referrersDescs.Manifests) > limit {
		return nil, nil, nil, fmt.Errorf("failed to fetch referrers: too many referrers found, max limit is %d", limit)
	}
	for _, manifestDesc := range referrersDescs.Manifests {
		// registries without the referrers API serve a fallback tag index whose descriptors may not
		// carry the artifact type, so those referrers are checked against their own manifest
		untyped := manifestDesc.ArtifactType == "" || manifestDesc.ArtifactType == ociEmptyArtifactType
		if !untyped && !isSigstoreBundleType(manifestDesc.ArtifactType) {
			continue
		}
		refImg, err := remote.Image(ref.Context().Digest(manifestDesc.Digest.String()), remoteOpts...)
		if err != nil {
			// a fallback tag index is not cleaned up when a referrer is deleted, so an entry whose
			// manifest is gone is skipped instead of failing the whole lookup
			if isNotFound(err) {
				logger.V(4).Info("skipping missing referrer", "digest", manifestDesc.Digest.String())
				missing = append(missing, fmt.Errorf("referrer %s not found: %w", manifestDesc.Digest, err))
				continue
			}
			return nil, nil, nil, fmt.Errorf("failed to fetch referrer image: %w", err)
		}
		if untyped {
			manifest, err := refImg.Manifest()
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to fetch referrer manifest: %w", err)
			}
			if !isSigstoreBundleManifest(manifest) {
				continue
			}
		}
		layers, err := refImg.Layers()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to fetch referrer layer: %w", err)
		}
		if len(layers) == 0 {
			return nil, nil, nil, fmt.Errorf("layers not found")
		}
		layer := layers[0]
		bundleBytes, err := verifiers.ReadLayerLimited(layer, verifiers.PayloadLimit())
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to read referrer layer %s: %w", manifestDesc.Digest, err)
		}
		b := &bundle.Bundle{}
		err = b.UnmarshalJSON(bundleBytes)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
		}
		bundles = append(bundles, &verificationBundle{ProtoBundle: b})
	}
	if predicateType != "" {
		filteredBundles := make([]*verificationBundle, 0)
		for _, b := range bundles {
			dsseEnvelope := b.ProtoBundle.Bundle.GetDsseEnvelope()
			if dsseEnvelope != nil {
				if dsseEnvelope.PayloadType != "application/vnd.in-toto+json" {
					continue
				}
				var intotoStatement in_toto.Statement //nolint:staticcheck
				if err := json.Unmarshal(dsseEnvelope.Payload, &intotoStatement); err != nil {
					continue
				}

				if intotoStatement.PredicateType == predicateType {
					filteredBundles = append(filteredBundles, &verificationBundle{
						ProtoBundle:   b.ProtoBundle,
						DSSE_Envelope: &intotoStatement,
					})
				}
			}
		}
		return filteredBundles, desc, missing, nil
	}
	return bundles, desc, missing, nil
}

func buildPolicy(desc *v1.Descriptor, opts verifiers.Options) (verify.PolicyBuilder, error) {
	digest, err := hex.DecodeString(desc.Digest.Hex)
	if err != nil {
		return verify.PolicyBuilder{}, err
	}
	artifactDigestVerificationOption := verify.WithArtifactDigest(desc.Digest.Algorithm, digest)
	hasIssuer := opts.Issuer != "" || opts.IssuerRegExp != ""
	hasSubject := opts.Subject != "" || opts.SubjectRegExp != ""
	if hasIssuer && hasSubject {
		if opts.Key != "" {
			return verify.PolicyBuilder{}, fmt.Errorf("static key and certificate identity are mutually exclusive")
		}
		id, err := verify.NewShortCertificateIdentity(opts.Issuer, opts.IssuerRegExp, opts.Subject, opts.SubjectRegExp)
		if err != nil {
			return verify.PolicyBuilder{}, err
		}
		return verify.NewPolicy(artifactDigestVerificationOption, verify.WithCertificateIdentity(id)), nil
	}
	if opts.Key != "" {
		return verify.NewPolicy(artifactDigestVerificationOption, verify.WithKey()), nil
	}
	return verify.NewPolicy(artifactDigestVerificationOption), nil
}

func buildVerifyOptions(opts verifiers.Options) []verify.VerifierOption {
	var verifierOptions []verify.VerifierOption
	if !opts.IgnoreTlog {
		verifierOptions = append(verifierOptions, verify.WithTransparencyLog(1))
	}
	if !opts.IgnoreSCT {
		verifierOptions = append(verifierOptions, verify.WithObserverTimestamps(1))
	}
	if len(verifierOptions) == 0 {
		if opts.Key != "" {
			verifierOptions = append(verifierOptions, verify.WithNoObserverTimestamps())
		} else {
			verifierOptions = append(verifierOptions, verify.WithCurrentTime())
		}
	}
	return verifierOptions
}

func getTrustedMaterial(ctx context.Context, opts verifiers.Options) (root.TrustedMaterial, error) {
	hasIdentity := opts.Issuer != "" || opts.IssuerRegExp != "" || opts.Subject != "" || opts.SubjectRegExp != ""
	if opts.Key != "" {
		if hasIdentity {
			return nil, fmt.Errorf("static key and certificate identity are mutually exclusive")
		}
		keyMaterial, err := buildKeyTrustedMaterial(ctx, opts.Key, opts.SignatureAlgorithm)
		if err != nil {
			return nil, err
		}
		if opts.IgnoreTlog && opts.IgnoreSCT {
			return keyMaterial, nil
		}
		trustedRoot, err := getTrustedRoot(ctx)
		if err != nil {
			return nil, err
		}
		return root.TrustedMaterialCollection{keyMaterial, trustedRoot}, nil
	}
	return getTrustedRoot(ctx)
}

func buildKeyTrustedMaterial(ctx context.Context, key, algorithm string) (root.TrustedMaterial, error) {
	hashAlgorithm, ok := signatureAlgorithmMap[algorithm]
	if !ok {
		return nil, fmt.Errorf("unsupported signature algorithm %q", algorithm)
	}
	var verifier signature.Verifier
	var err error
	if strings.Contains(key, "PUBLIC KEY") {
		verifier, err = decodePEM([]byte(key), hashAlgorithm)
	} else {
		verifier, err = sigs.PublicKeyFromKeyRefWithHashAlgo(ctx, key, hashAlgorithm)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}
	expiringKey := root.NewExpiringKey(verifier, time.Time{}, time.Time{})
	return root.NewTrustedPublicKeyMaterial(func(string) (root.TimeConstrainedVerifier, error) {
		return expiringKey, nil
	}), nil
}

func getTrustedRoot(ctx context.Context) (*root.TrustedRoot, error) {
	return sigstoretuf.TrustedRoot(ctx)
}

func decodeStatementsFromBundles(bundles []*verificationResult) ([]map[string]any, error) {
	if len(bundles) == 0 {
		return []map[string]any{}, nil
	}
	var err error
	var statement map[string]any
	var intotostatement in_toto.Statement //nolint:staticcheck
	decodedStatements := make([]map[string]any, len(bundles))
	for i, b := range bundles {
		intotostatement = *b.Bundle.DSSE_Envelope
		statement, err = data.ToMap(intotostatement)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode statement: %v", intotostatement.Type)
		}
		statement["type"] = intotostatement.PredicateType
		decodedStatements[i] = statement
	}
	return decodedStatements, nil
}
