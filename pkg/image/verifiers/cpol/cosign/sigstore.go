package cosign

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

var (
	maxLayerSize     = int64(10 * 1000 * 1000) // 10 MB
	attestationlimit = 50
	// maxProbeLayerSize bounds a single untyped layer read only to check whether it holds a bundle.
	// Sigstore bundles are a few KB, so this keeps unrelated referrers cheap to skip.
	maxProbeLayerSize = int64(1000 * 1000) // 1 MB
	// maxProbeTotalSize bounds the untyped layer bytes probed across all referrers of one image.
	maxProbeTotalSize = int64(5 * 1000 * 1000) // 5 MB
)

const (
	sigstoreBundleArtifactType = "application/vnd.dev.sigstore.bundle"
	ociEmptyArtifactType       = "application/vnd.oci.empty.v1+json"
)

// nonBundleLayerMediaTypePrefixes are layer media types that never hold a sigstore bundle, so the
// fallback probe skips them without reading the layer.
var nonBundleLayerMediaTypePrefixes = []string{
	"application/vnd.oci.image.layer.",
	"application/vnd.docker.image.rootfs.",
	"application/vnd.dev.cosign.",
	"application/vnd.dsse.envelope.",
	"application/vnd.in-toto",
	"application/spdx",
	"application/vnd.cyclonedx",
}

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
	bundles, desc, err := fetchBundles(ref, attestationlimit, opts.Type, remoteOpts)
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

func isSigstoreBundleType(mediaType string) bool {
	return strings.HasPrefix(mediaType, sigstoreBundleArtifactType)
}

// readLayer reads the uncompressed content of a layer, failing when either the compressed size or the
// uncompressed content exceeds limit. On failure it still returns what was read, for accounting.
func readLayer(layer v1.Layer, limit int64) ([]byte, error) {
	layerSize, err := layer.Size()
	if err != nil {
		return nil, err
	}
	if layerSize > limit {
		return nil, fmt.Errorf("layer size %d exceeds %d", layerSize, limit)
	}
	layerBytes, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch referrer layer: %w", err)
	}
	defer layerBytes.Close()
	// the compressed size does not bound the uncompressed stream, so cap the read as well
	data, err := io.ReadAll(io.LimitReader(layerBytes, limit+1))
	if err != nil {
		return data, fmt.Errorf("failed to fetch referrer layer: %w", err)
	}
	if int64(len(data)) > limit {
		return data, fmt.Errorf("uncompressed layer size exceeds %d", limit)
	}
	return data, nil
}

// resolveFallbackBundle checks whether an untyped referrer holds a sigstore bundle, first from its
// manifest artifact type and first layer media type, then by parsing a small first layer. It returns
// the referrer image, plus the bundle content when it was read, or nil when it is not a bundle.
func resolveFallbackBundle(ref name.Reference, remoteOpts []remote.Option, probeBudget *int64) (v1.Image, []byte) {
	img, err := remote.Image(ref, remoteOpts...)
	if err != nil || img == nil {
		return nil, nil
	}
	manifest, err := img.Manifest()
	if err != nil || manifest == nil {
		return nil, nil
	}
	if isSigstoreBundleType(manifest.ArtifactType) {
		return img, nil
	}
	if len(manifest.Layers) == 0 {
		return nil, nil
	}
	layerDesc := manifest.Layers[0]
	if isSigstoreBundleType(string(layerDesc.MediaType)) {
		return img, nil
	}
	for _, prefix := range nonBundleLayerMediaTypePrefixes {
		if strings.HasPrefix(string(layerDesc.MediaType), prefix) {
			return nil, nil
		}
	}
	limit := min(maxProbeLayerSize, *probeBudget)
	if limit <= 0 || layerDesc.Size > limit {
		return nil, nil
	}
	layers, err := img.Layers()
	if err != nil || len(layers) == 0 {
		return nil, nil
	}
	data, err := readLayer(layers[0], limit)
	// charge the bytes actually read, since the descriptor size does not bound the uncompressed content
	*probeBudget -= max(int64(len(data)), layerDesc.Size)
	if err != nil {
		return nil, nil
	}
	b := &bundle.Bundle{}
	if err := b.UnmarshalJSON(data); err != nil || b.Bundle == nil {
		return nil, nil
	}
	return img, data
}

func fetchBundles(ref name.Reference, limit int, predicateType string, remoteOpts []remote.Option) ([]*verificationBundle, *v1.Descriptor, error) {
	bundles := make([]*verificationBundle, 0)
	desc, err := remote.Head(ref, remoteOpts...)
	if err != nil {
		return nil, nil, err
	}
	referrers, err := remote.Referrers(ref.Context().Digest(desc.Digest.String()), remoteOpts...)
	if err != nil {
		return nil, nil, err
	}
	referrersDescs, err := referrers.IndexManifest()
	if err != nil {
		return nil, nil, err
	}
	if len(referrersDescs.Manifests) > limit {
		return nil, nil, fmt.Errorf("failed to fetch referrers: too many referrers found, max limit is %d", limit)
	}
	// remaining bytes the fallback probe may read across all referrers of this image
	probeBudget := maxProbeTotalSize
	for _, manifestDesc := range referrersDescs.Manifests {
		artifactType := manifestDesc.ArtifactType
		var refImg v1.Image
		var bundleBytes []byte

		// registries without the referrers API serve a fallback tag index whose descriptors may not
		// carry the artifact type, so resolve it from the referrer itself
		if !isSigstoreBundleType(artifactType) && (artifactType == "" || artifactType == ociEmptyArtifactType) {
			refImg, bundleBytes = resolveFallbackBundle(ref.Context().Digest(manifestDesc.Digest.String()), remoteOpts, &probeBudget)
			if refImg != nil {
				artifactType = sigstoreBundleArtifactType
			}
		}

		if !isSigstoreBundleType(artifactType) {
			continue
		}

		if refImg == nil {
			var err error
			refImg, err = remote.Image(ref.Context().Digest(manifestDesc.Digest.String()), remoteOpts...)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to fetch referrer image: %w", err)
			}
		}
		if bundleBytes == nil {
			layers, err := refImg.Layers()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to fetch referrer layer: %w", err)
			}
			if len(layers) == 0 {
				return nil, nil, fmt.Errorf("layers not found")
			}
			bundleBytes, err = readLayer(layers[0], maxLayerSize)
			if err != nil {
				return nil, nil, err
			}
		}

		b := &bundle.Bundle{}
		err = b.UnmarshalJSON(bundleBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
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
		return filteredBundles, desc, nil
	}
	return bundles, desc, nil
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
