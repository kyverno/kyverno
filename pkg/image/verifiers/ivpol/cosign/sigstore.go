package cosign

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// tsaOnlyTrustedMaterial wraps a SigstoreTimestampingAuthority as a
// TrustedMaterial that only exposes the TSA. It is used to compose with a
// public TUF-managed TrustedRoot via TrustedMaterialCollection when the
// caller's TSA chain is not part of any TUF root — e.g. GitHub's per-tenant
// TSA used by actions/attest-build-provenance on private repos.
//
// All other TrustedMaterial methods (Fulcio CAs, Rekor logs, CT logs)
// inherit empty defaults from BaseTrustedMaterial; the public-root member of
// the surrounding TrustedMaterialCollection provides those.
type tsaOnlyTrustedMaterial struct {
	root.BaseTrustedMaterial
	tsa *root.SigstoreTimestampingAuthority
}

func (t *tsaOnlyTrustedMaterial) TimestampingAuthorities() []root.TimestampingAuthority {
	return []root.TimestampingAuthority{t.tsa}
}

// composeTrustedMaterial returns a TrustedMaterial suitable for sigstore-go's
// verify.NewVerifier.
//
// If customTSALeaf is nil, the public root is returned unchanged (no custom
// TSA configured).
//
// If customTSALeaf is non-nil, the leaf + intermediates + (first) root are
// composed with the public root in a TrustedMaterialCollection so sigstore-
// go's verifier sees timestamping authorities from both sources.
//
// This is the function that closes the v2->v3 cosign-as-library regression
// described in sigstore/cosign#4847: callers configuring a TSA cert chain
// alongside a TUF-fetched trusted root should have their TSA honoured.
func composeTrustedMaterial(publicRoot *root.TrustedRoot, customTSALeaf *x509.Certificate, customTSAIntermediates []*x509.Certificate, customTSARoots []*x509.Certificate) (root.TrustedMaterial, error) {
	if publicRoot == nil {
		return nil, errors.New("public trusted root must not be nil")
	}
	if customTSALeaf == nil {
		return publicRoot, nil
	}
	if len(customTSARoots) == 0 {
		return nil, errors.New("custom TSA chain requires at least one root certificate")
	}
	customTSA := &root.SigstoreTimestampingAuthority{
		Root:          customTSARoots[0],
		Intermediates: customTSAIntermediates,
		Leaf:          customTSALeaf,
	}
	return root.TrustedMaterialCollection{
		publicRoot,
		&tsaOnlyTrustedMaterial{tsa: customTSA},
	}, nil
}

// dispatchVerify routes a VerifyImageSignature call into the appropriate
// underlying verification engine based on the CheckOpts shape:
//
//   - bundle format + keyless (cOpts.TrustedMaterial is *root.TrustedRoot) →
//     sigstore-go directly via verifyImageBundleAttestations. This is the
//     path that closes the v2->v3 cosign-as-library TSA-chain regression for
//     callers (e.g. GitHub Artifact Attestations on private repos).
//   - bundle format + static key (cOpts.SigVerifier set) →
//     cosign.VerifyImageAttestations. cosign handles static-key bundle
//     verification via co.SigVerifier and is unaffected by the TSA-chain
//     regression (no TSA chain is set on this path).
//   - non-bundle → cosign.VerifyImageSignatures (legacy path, unchanged).
//
// Routing decisions live here rather than at the call site so the same logic
// is shared between VerifyImageSignature and VerifyAttestationSignature.
func dispatchVerify(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	if co.NewBundleFormat {
		if pr, ok := co.TrustedMaterial.(*root.TrustedRoot); ok && pr != nil {
			return verifyImageBundleAttestations(ctx, signedImgRef, co, pr)
		}
		// Static-key (or other non-keyless) bundle path: cosign handles this.
		return cosign.VerifyImageAttestations(ctx, signedImgRef, co)
	}
	return cosign.VerifyImageSignatures(ctx, signedImgRef, co)
}

// dispatchVerifyAttestations is the attestation-flavoured sibling of
// dispatchVerify. The only difference is the non-bundle fallback uses
// VerifyImageAttestations rather than VerifyImageSignatures.
func dispatchVerifyAttestations(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	if co.NewBundleFormat {
		if pr, ok := co.TrustedMaterial.(*root.TrustedRoot); ok && pr != nil {
			return verifyImageBundleAttestations(ctx, signedImgRef, co, pr)
		}
	}
	return cosign.VerifyImageAttestations(ctx, signedImgRef, co)
}

// verifyImageBundleAttestations is the IVPOL bundle-format entry point. It
// fetches all attached sigstore protobuf bundles for the image and verifies
// each one against the composed trusted material using sigstore-go directly.
//
// This bypasses cosign.VerifyImageAttestations (which has the v2->v3
// regression on the legacy TSA fields, sigstore/cosign#4849) and lets us
// honour both TrustedMaterial and a caller-provided TSA chain.
//
// The TSA cert chain (if any) is read from co.TSACertificate /
// TSAIntermediateCertificates / TSARootCertificates — populated by
// checkOptions when the attestor declares CTLog.TSACertChain. We read these
// directly rather than re-parsing the PEM so parsing only happens once.
//
// The returned slice has one oci.Signature per verified bundle, mirroring
// the cosign API shape so the caller (verifier.go) can keep its post-
// verification annotation/identity-matching logic unchanged.
func verifyImageBundleAttestations(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts, publicRoot *root.TrustedRoot) ([]oci.Signature, bool, error) {
	bundles, hash, err := cosign.GetBundles(ctx, signedImgRef, co.RegistryClientOpts)
	if err != nil {
		return nil, false, fmt.Errorf("fetching bundles: %w", err)
	}
	if len(bundles) == 0 {
		return nil, false, errors.New("no bundles attached to image")
	}

	tm, err := composeTrustedMaterial(publicRoot, co.TSACertificate, co.TSAIntermediateCertificates, co.TSARootCertificates)
	if err != nil {
		return nil, false, fmt.Errorf("composing trusted material: %w", err)
	}

	policy, err := buildBundlePolicy(hash, co)
	if err != nil {
		return nil, false, fmt.Errorf("building policy: %w", err)
	}

	verifierOpts := buildBundleVerifyOptions(co)
	verifier, err := verify.NewVerifier(tm, verifierOpts...)
	if err != nil {
		return nil, false, fmt.Errorf("creating sigstore-go verifier: %w", err)
	}

	atLeastOne := false
	results := make([]oci.Signature, 0, len(bundles))
	var combinedErrs []error
	for _, b := range bundles {
		_, vErr := verifier.Verify(b, policy)
		if vErr != nil {
			combinedErrs = append(combinedErrs, vErr)
			continue
		}
		sig, sErr := bundleToOCISignature(b)
		if sErr != nil {
			combinedErrs = append(combinedErrs, sErr)
			continue
		}
		atLeastOne = true
		results = append(results, sig)
	}

	if !atLeastOne {
		return nil, false, fmt.Errorf("no bundle verified: %w", errors.Join(combinedErrs...))
	}
	return results, true, nil
}

// buildBundlePolicy constructs the sigstore-go verify.PolicyBuilder for an
// IVPOL bundle verification call. It expects the image digest (which sigstore-
// go binds the verification to) and identity matchers from CheckOpts.
func buildBundlePolicy(hash *v1.Hash, co *cosign.CheckOpts) (verify.PolicyBuilder, error) {
	digestBytes, err := hex.DecodeString(hash.Hex)
	if err != nil {
		return verify.PolicyBuilder{}, fmt.Errorf("decoding digest: %w", err)
	}
	artifactOpt := verify.WithArtifactDigest(hash.Algorithm, digestBytes)

	if len(co.Identities) == 0 {
		return verify.NewPolicy(artifactOpt), nil
	}
	id := co.Identities[0]
	hasIssuer := id.Issuer != "" || id.IssuerRegExp != ""
	hasSubject := id.Subject != "" || id.SubjectRegExp != ""
	if !hasIssuer || !hasSubject {
		return verify.NewPolicy(artifactOpt), nil
	}
	certID, err := verify.NewShortCertificateIdentity(id.Issuer, id.IssuerRegExp, id.Subject, id.SubjectRegExp)
	if err != nil {
		return verify.PolicyBuilder{}, fmt.Errorf("building certificate identity: %w", err)
	}
	return verify.NewPolicy(artifactOpt, verify.WithCertificateIdentity(certID)), nil
}

// buildBundleVerifyOptions translates IVPOL's CheckOpts into sigstore-go
// VerifierOptions.
//
// sigstore-go's VerifierConfig requires exactly one "time" option from the
// set {WithSignedTimestamps, WithObserverTimestamps, WithIntegratedTimestamps,
// WithCurrentTime, WithNoObserverTimestamps}. The matrix below mirrors
// cosign's pkg/cosign/verify.go verificationOptions to preserve behaviour
// for callers migrating from the cosign-as-library path.
func buildBundleVerifyOptions(co *cosign.CheckOpts) []verify.VerifierOption {
	var opts []verify.VerifierOption

	if !co.IgnoreTlog {
		opts = append(opts, verify.WithTransparencyLog(1))
	}
	if !co.IgnoreSCT {
		opts = append(opts, verify.WithSignedCertificateTimestamps(1))
	}

	// Choose exactly one timestamp option. UseSignedTimestamps takes
	// precedence (it expresses an explicit caller requirement). Otherwise
	// prefer log-integrated timestamps if Rekor is in scope. Fall back to
	// WithCurrentTime (cert-based) or WithNoObserverTimestamps (static key)
	// when the caller has opted out of all timestamp infrastructure.
	switch {
	case co.UseSignedTimestamps:
		opts = append(opts, verify.WithSignedTimestamps(1))
	case !co.IgnoreTlog:
		opts = append(opts, verify.WithIntegratedTimestamps(1))
	case co.SigVerifier != nil:
		opts = append(opts, verify.WithNoObserverTimestamps())
	default:
		opts = append(opts, verify.WithCurrentTime())
	}

	return opts
}

// bundleToOCISignature wraps a verified sigstore protobuf bundle as an
// oci.Signature carrying the DSSE-envelope JSON payload, matching the shape
// cosign's verifyImageAttestationsSigstoreBundle returns. The IVPOL
// verifier.go post-verification logic (annotation/predicate matching,
// length checks) treats the returned []oci.Signature exactly the same way
// regardless of which engine produced it.
//
// Only the payload is wrapped today, matching cosign's note that additional
// data (cert chain, rekor/tsa) requires upstream sigstore-go work
// (sigstore-go#328).
func bundleToOCISignature(b *sgbundle.Bundle) (oci.Signature, error) {
	if b == nil {
		return nil, errors.New("nil bundle")
	}
	if b.Bundle == nil {
		return nil, errors.New("bundle has no inner protobuf content")
	}
	dsseEnvelope, ok := b.Bundle.Content.(*protobundle.Bundle_DsseEnvelope)
	if !ok {
		return nil, errors.New("bundle does not contain a DSSE envelope")
	}
	payload, err := json.Marshal(dsseEnvelope.DsseEnvelope)
	if err != nil {
		return nil, fmt.Errorf("marshaling DSSE envelope: %w", err)
	}
	return static.NewAttestation(payload)
}
