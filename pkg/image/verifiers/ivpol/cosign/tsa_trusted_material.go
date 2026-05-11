package cosign

import (
	"crypto/x509"
	"errors"

	"github.com/sigstore/sigstore-go/pkg/root"
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

// mergeTSAIntoTrustedMaterial returns a TrustedMaterial that exposes both
// the public trust root's timestamping authorities and a caller-supplied
// TSA chain. When the caller supplies no custom material (leaf, intermediates,
// and roots all empty), the public root is returned unchanged.
//
// The merged material reaches sigstore-go's bundle verifier when cosign's
// pkg/cosign.verificationOptions wraps co.TrustedMaterial — the verifier
// looks up timestamping authorities through the wrapper, and a
// TrustedMaterialCollection's TimestampingAuthorities aggregates from all
// its members.
//
// SigstoreTimestampingAuthority has a single Root field, so each root in the
// caller's slice gets its own authority entry (all sharing the same leaf and
// intermediates). The leaf may be nil — sigstore-go's verifier accepts a nil
// Leaf and pulls the signing cert from the TSR when present, or errors
// clearly when neither is available. This matches cpol/cosign's non-bundle
// path (which accepts leaf-less, multi-root TSA chains) and means callers
// don't need to know whether their bundle uses the legacy or new format.
func mergeTSAIntoTrustedMaterial(publicRoot root.TrustedMaterial, customTSALeaf *x509.Certificate, customTSAIntermediates, customTSARoots []*x509.Certificate) (root.TrustedMaterial, error) {
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
