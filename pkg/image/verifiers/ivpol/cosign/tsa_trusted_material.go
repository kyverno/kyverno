package cosign

import (
	"crypto/x509"
	"errors"
	"fmt"

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
// TSA chain. With no leaf, the public root is returned unchanged (no
// custom TSA configured).
//
// The merged material reaches sigstore-go's bundle verifier when cosign's
// pkg/cosign.verificationOptions wraps co.TrustedMaterial — the verifier
// looks up timestamping authorities through the wrapper, and a
// TrustedMaterialCollection's TimestampingAuthorities aggregates from all
// its members.
//
// roots must contain exactly zero or one cert: SigstoreTimestampingAuthority
// has a single Root field, and silently truncating a longer slice would hide
// a chain that mixes multiple trust anchors.
func mergeTSAIntoTrustedMaterial(publicRoot root.TrustedMaterial, leaf *x509.Certificate, intermediates, roots []*x509.Certificate) (root.TrustedMaterial, error) {
	if publicRoot == nil {
		return nil, errors.New("public trusted material must not be nil")
	}
	if leaf == nil {
		return publicRoot, nil
	}
	switch len(roots) {
	case 0:
		return nil, errors.New("custom TSA chain requires at least one root certificate")
	case 1:
		// fine
	default:
		return nil, fmt.Errorf("custom TSA chain must have exactly one root certificate (SigstoreTimestampingAuthority's Root is single-valued), got %d", len(roots))
	}
	customTSA := &root.SigstoreTimestampingAuthority{
		Root:          roots[0],
		Intermediates: intermediates,
		Leaf:          leaf,
	}
	return root.TrustedMaterialCollection{
		publicRoot,
		&tsaOnlyTrustedMaterial{tsa: customTSA},
	}, nil
}
