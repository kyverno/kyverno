package cosign

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/tuf"
)

var (
	maxLayerSize     = int64(10 * 1000 * 1000) // 10 MB
	attestationlimit = 50
)

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
	remoteOpts, err := opts.Client.Options(ctx)
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
	trustedMaterial, err := getTrustedRoot(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get trusted root: %v", opts.ImageRef)
	}
	results, err := verifyBundles(bundles, desc, trustedMaterial, policy, verifyOpts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get verify bundles: %v", opts.ImageRef)
	}
	return results, nil
}

func verifyBundles(bundles []*verificationBundle, desc *v1.Descriptor, trustedRoot *root.TrustedRoot, policy verify.PolicyBuilder, verifierOpts []verify.VerifierOption) ([]*verificationResult, error) {
	verifier, err := verify.NewSignedEntityVerifier(trustedRoot, verifierOpts...)
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
	for _, manifestDesc := range referrersDescs.Manifests {
		if !strings.HasPrefix(manifestDesc.ArtifactType, "application/vnd.dev.sigstore.bundle") {
			continue
		}
		refImg, err := remote.Image(ref.Context().Digest(manifestDesc.Digest.String()), remoteOpts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch referrer image: %w", err)
		}
		layers, err := refImg.Layers()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch referrer layer: %w", err)
		}
		if len(layers) == 0 {
			return nil, nil, fmt.Errorf("layers not found")
		}
		layer := layers[0]
		layerSize, err := layer.Size()
		if err != nil {
			return nil, nil, err
		}
		if layerSize > maxLayerSize {
			return nil, nil, fmt.Errorf("layer size %d exceeds %d", layerSize, maxLayerSize)
		}
		layerBytes, err := layer.Uncompressed()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch referrer layer: %w", err)
		}
		defer layerBytes.Close()
		bundleBytes, err := io.ReadAll(layerBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch referrer layer: %w", err)
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
		id, err := verify.NewShortCertificateIdentity(opts.Issuer, opts.IssuerRegExp, opts.Subject, opts.SubjectRegExp)
		if err != nil {
			return verify.PolicyBuilder{}, err
		}
		return verify.NewPolicy(artifactDigestVerificationOption, verify.WithCertificateIdentity(id)), nil
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
	return verifierOptions
}

func getTrustedRoot(ctx context.Context) (*root.TrustedRoot, error) {
	tufClient, err := tuf.NewFromEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("initializing tuf: %w", err)
	}
	targetBytes, err := tufClient.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("error getting targets: %w", err)
	}
	trustedRoot, err := root.NewTrustedRootFromJSON(targetBytes)
	if err != nil {
		return nil, fmt.Errorf("error creating trusted root: %w", err)
	}
	return trustedRoot, nil
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
