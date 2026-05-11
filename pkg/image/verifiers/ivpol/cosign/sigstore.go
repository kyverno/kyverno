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

// tsaOnlyTrustedMaterial exposes a single SigstoreTimestampingAuthority via
// the TrustedMaterial interface. Compose it with a TUF-managed TrustedRoot
// in a TrustedMaterialCollection when the caller's TSA chain isn't part of
// any TUF root (e.g. GitHub's per-tenant TSA). Other interface methods
// inherit empty defaults from BaseTrustedMaterial; the public-root member
// of the surrounding collection contributes those.
type tsaOnlyTrustedMaterial struct {
	root.BaseTrustedMaterial
	tsa *root.SigstoreTimestampingAuthority
}

func (t *tsaOnlyTrustedMaterial) TimestampingAuthorities() []root.TimestampingAuthority {
	return []root.TimestampingAuthority{t.tsa}
}

// composeTrustedMaterial returns a TrustedMaterial that exposes both the
// public trust root's timestamping authorities and a caller-supplied TSA
// chain (for a TSA not present in any TUF root).
//
// When the entire custom chain is empty (no leaf, no intermediates, no
// roots) the public root is returned unchanged. When roots are provided,
// one SigstoreTimestampingAuthority is appended to the collection per root,
// each sharing the (possibly nil) leaf and the intermediates. This mirrors
// the multi-root semantics of cosign's TSACertChain handling on the
// non-bundle path (cpol/cosign/cosign.go:179-201 likewise passes the whole
// roots slice through) — restricting multi-root chains only on this path
// would introduce a behavioural divergence we don't want.
//
// A nil leaf is honoured: sigstore-go's SigstoreTimestampingAuthority.Verify
// delegates to tsaverification.VerifyTimestampResponse, which documents
// TSACertificate as "Optional if the TSR contains the TSA certificate" and
// errors with a clear "leaf certificate must be present in the TSR or as a
// verify option" if neither is available. This matches the TSACertChain
// API contract that the leaf may live in the RFC3161 timestamp itself.
func composeTrustedMaterial(publicRoot root.TrustedMaterial, customTSALeaf *x509.Certificate, customTSAIntermediates, customTSARoots []*x509.Certificate) (root.TrustedMaterial, error) {
	if publicRoot == nil {
		return nil, errors.New("public trusted material must not be nil")
	}
	if customTSALeaf == nil && len(customTSAIntermediates) == 0 && len(customTSARoots) == 0 {
		return publicRoot, nil
	}
	if len(customTSARoots) == 0 {
		return nil, errors.New("custom TSA chain requires at least one root certificate")
	}
	members := root.TrustedMaterialCollection{publicRoot}
	for _, r := range customTSARoots {
		members = append(members, &tsaOnlyTrustedMaterial{
			tsa: &root.SigstoreTimestampingAuthority{
				Root:          r,
				Intermediates: customTSAIntermediates,
				Leaf:          customTSALeaf,
			},
		})
	}
	return members, nil
}

// rejectAnnotationsOnBundlePath errors when the caller configured annotation
// matchers on an attestor whose CheckOpts will route through any bundle path
// — keyless via this file's sigstore-go helper, or static-key via cosign's
// verifyImageAttestationsSigstoreBundle. Both paths return oci.Signature
// values built from the DSSE envelope only (via static.NewAttestation), so
// the resulting signatures carry no OCI annotations. Without this guard the
// surrounding annotation-check loop in verifier.go would always fail with
// "no signature matched the required annotations", which is a misleading
// framing of "this combination of attestor fields isn't supported".
//
// Gated on co.NewBundleFormat rather than isBundleKeyless because the
// limitation is bundle-format-specific, not keyless-specific.
func rejectAnnotationsOnBundlePath(co *cosign.CheckOpts, annotations map[string]string) error {
	if !co.NewBundleFormat || len(annotations) == 0 {
		return nil
	}
	return errors.New("attestor.cosign.annotations is not supported on the v3-bundle verification path: OCI annotations are not propagated from sigstore bundle referrers (matches cosign's own behaviour for the bundle path)")
}

// isBundleKeyless answers "should this CheckOpts go through our sigstore-go
// path rather than cosign?". True when bundle format is in use, no static
// public key is set (which would route through cosign's own bundle handler),
// and we have non-nil trust material to compose against.
//
// Keyed on co.SigVerifier rather than asserting co.TrustedMaterial's concrete
// type: opts.go is the single producer of TrustedMaterial and may legitimately
// wrap it (the call site below also wraps it inside verifyImageBundleAttestations
// via composeTrustedMaterial → TrustedMaterialCollection). Gating on a
// concrete-type assertion would silently route around the bug-fix path the
// first time TrustedMaterial isn't a bare *root.TrustedRoot.
func isBundleKeyless(co *cosign.CheckOpts) bool {
	return co.NewBundleFormat && co.SigVerifier == nil && co.TrustedMaterial != nil
}

// dispatchVerify routes signature verification by CheckOpts shape:
// bundle + keyless goes through sigstore-go directly so a caller-provided
// TSA chain composes into the trusted material; bundle + static key goes
// through cosign (no TSA chain to honour); non-bundle keeps the legacy
// cosign path.
func dispatchVerify(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	if isBundleKeyless(co) {
		return verifyImageBundleAttestations(ctx, signedImgRef, co, co.TrustedMaterial)
	}
	if co.NewBundleFormat {
		return cosign.VerifyImageAttestations(ctx, signedImgRef, co)
	}
	return cosign.VerifyImageSignatures(ctx, signedImgRef, co)
}

// dispatchVerifyAttestations is dispatchVerify's sibling for the attestation
// flow; the only difference is the non-bundle fallback uses
// cosign.VerifyImageAttestations rather than cosign.VerifyImageSignatures.
func dispatchVerifyAttestations(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	if isBundleKeyless(co) {
		return verifyImageBundleAttestations(ctx, signedImgRef, co, co.TrustedMaterial)
	}
	return cosign.VerifyImageAttestations(ctx, signedImgRef, co)
}

// verifyImageBundleAttestations fetches and verifies all attached sigstore
// protobuf bundles using sigstore-go directly.
//
// The TSA cert chain (if any) is read from the legacy CheckOpts fields
// (TSACertificate / TSAIntermediateCertificates / TSARootCertificates),
// which checkOptions populated from CTLog.TSACertChain — re-using the
// already-parsed certs rather than re-parsing the PEM.
func verifyImageBundleAttestations(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts, publicRoot root.TrustedMaterial) ([]oci.Signature, bool, error) {
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

	verifier, err := verify.NewVerifier(tm, buildBundleVerifyOptions(co)...)
	if err != nil {
		return nil, false, fmt.Errorf("creating sigstore-go verifier: %w", err)
	}

	atLeastOne := false
	results := make([]oci.Signature, 0, len(bundles))
	var combinedErrs []error
	for _, b := range bundles {
		if _, vErr := verifier.Verify(b, policy); vErr != nil {
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

// buildBundlePolicy builds the sigstore-go PolicyBuilder bound to the image
// digest and the configured identities (OR-evaluated by sigstore-go).
func buildBundlePolicy(hash *v1.Hash, co *cosign.CheckOpts) (verify.PolicyBuilder, error) {
	digestBytes, err := hex.DecodeString(hash.Hex)
	if err != nil {
		return verify.PolicyBuilder{}, fmt.Errorf("decoding digest: %w", err)
	}
	artifactOpt := verify.WithArtifactDigest(hash.Algorithm, digestBytes)

	policyOpts, err := certificateIdentityOptions(co.Identities)
	if err != nil {
		return verify.PolicyBuilder{}, err
	}
	return verify.NewPolicy(artifactOpt, policyOpts...), nil
}

// certificateIdentityOptions returns one WithCertificateIdentity per
// Identity. sigstore-go's CertificateIdentities.Verify is OR, preserving
// cosign's keyless-with-multiple-identities semantics.
//
// sigstore-go requires every identity to configure both an issuer matcher
// (Issuer or IssuerRegExp) and a SAN matcher (Subject or SubjectRegExp);
// half-specified entries error out of NewShortCertificateIdentity. The
// error is propagated rather than silently skipped — silent skipping
// would let a misconfigured entry weaken verification to digest-only
// without any signal back to the operator.
func certificateIdentityOptions(identities []cosign.Identity) ([]verify.PolicyOption, error) {
	opts := make([]verify.PolicyOption, 0, len(identities))
	for _, id := range identities {
		certID, err := verify.NewShortCertificateIdentity(id.Issuer, id.IssuerRegExp, id.Subject, id.SubjectRegExp)
		if err != nil {
			return nil, fmt.Errorf("building certificate identity for issuer=%q issuerRegExp=%q subject=%q subjectRegExp=%q: %w",
				id.Issuer, id.IssuerRegExp, id.Subject, id.SubjectRegExp, err)
		}
		opts = append(opts, verify.WithCertificateIdentity(certID))
	}
	return opts, nil
}

// buildBundleVerifyOptions mirrors cosign's pkg/cosign/verify.go option
// matrix. sigstore-go requires exactly one of WithSignedTimestamps,
// WithObserverTimestamps, WithIntegratedTimestamps, WithCurrentTime, or
// WithNoObserverTimestamps; pick the one that matches the caller's
// (UseSignedTimestamps, IgnoreTlog, SigVerifier) triple.
func buildBundleVerifyOptions(co *cosign.CheckOpts) []verify.VerifierOption {
	var opts []verify.VerifierOption

	if !co.IgnoreTlog {
		opts = append(opts, verify.WithTransparencyLog(1))
	}
	if !co.IgnoreSCT {
		opts = append(opts, verify.WithSignedCertificateTimestamps(1))
	}

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

// bundleToOCISignature wraps a verified bundle's DSSE envelope as an
// oci.Signature so the caller's post-verification logic is engine-agnostic.
// Only the payload is carried; the cert chain and timestamp data on the
// bundle are not propagated.
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
