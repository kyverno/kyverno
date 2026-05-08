package cosign

import (
	"crypto/x509"

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
// the public TrustedRoot's timestamping authorities and a caller-supplied
// TSA chain. With no leaf or no roots it returns the public root unchanged
// so callers without a custom TSA configured see no behavioural change.
//
// The merged material reaches sigstore-go's bundle verifier when cosign's
// pkg/cosign.verificationOptions wraps co.TrustedMaterial — the verifier
// looks up timestamping authorities through the wrapper, and a
// TrustedMaterialCollection's TimestampingAuthorities aggregates from all
// its members.
func mergeTSAIntoTrustedMaterial(publicRoot *root.TrustedRoot, leaf *x509.Certificate, intermediates, roots []*x509.Certificate) root.TrustedMaterial {
	if leaf == nil || len(roots) == 0 {
		return publicRoot
	}
	customTSA := &root.SigstoreTimestampingAuthority{
		Root:          roots[0],
		Intermediates: intermediates,
		Leaf:          leaf,
	}
	return root.TrustedMaterialCollection{
		publicRoot,
		&tsaOnlyTrustedMaterial{tsa: customTSA},
	}
}
