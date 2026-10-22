package sigstoreguard

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore-go/pkg/root"
	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
)

// VerificationMaterial contains the trust used for both legacy signatures and
// modern bundles. All repository material comes from the same TUF trust anchor.
type VerificationMaterial struct {
	// TrustedRoot is nil for authenticated PEM-only operator repositories.
	TrustedRoot         *root.TrustedRoot
	RekorPublicKeys     *cosign.TrustedTransparencyLogPubKeys
	CTLogPublicKeys     *cosign.TrustedTransparencyLogPubKeys
	FulcioRoots         *x509.CertPool
	FulcioIntermediates *x509.CertPool
}

// VerificationMaterialFor preserves operator key-file overrides and legacy PEM
// targets, falling back to trusted_root.json when individual targets are absent.
// Operator repositories may omit the modern target when authenticated metadata
// proves it is absent. Explicit policy mirrors and roots still require that target
// and cannot read or overwrite the operator's TUF cache.
func VerificationMaterialFor(ctx context.Context, mirror string, rootBytes []byte) (*VerificationMaterial, error) {
	return verificationMaterialFor(ctx, mirror, rootBytes, registryclient.EgressHTTPClient())
}

func verificationMaterialFor(ctx context.Context, mirror string, rootBytes []byte, httpClient *http.Client) (*VerificationMaterial, error) {
	isolate := mirror != "" || len(rootBytes) != 0
	if !isolate {
		mirror, rootBytes = defaultRepository()
		if err := lockSharedCache(ctx); err != nil {
			return nil, err
		}
		defer unlockSharedCache()
	}
	opts, err := tufOptions(ctx, mirror, rootBytes, true, httpClient, isolate)
	if err != nil {
		return nil, err
	}
	client, err := newUpdaterClient(opts)
	if err != nil {
		return nil, fmt.Errorf("initializing TUF: %w", err)
	}
	tr, err := trustedRootFromRepository(client, !isolate)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	material, err := verificationMaterialFromTargets(tr, client)
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	return material, err
}

func trustedRootFromRepository(client *legacyTUFClient, allowAbsent bool) (*root.TrustedRoot, error) {
	const targetName = "trusted_root.json"
	var data []byte
	var err error
	if allowAbsent {
		var present bool
		data, present, err = client.optionalTarget(targetName)
		if err == nil && !present {
			return nil, nil
		}
	} else {
		data, err = client.GetTarget(targetName)
	}
	if err != nil {
		return nil, fmt.Errorf("getting trusted_root.json from TUF: %w", err)
	}
	tr, err := root.NewTrustedRootFromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parsing trusted root: %w", err)
	}
	return tr, nil
}

func verificationMaterialFromTargets(tr *root.TrustedRoot, client *legacyTUFClient) (*VerificationMaterial, error) {
	material := &VerificationMaterial{TrustedRoot: tr}
	var err error
	material.RekorPublicKeys, err = repositoryPublicKeys(client, "SIGSTORE_REKOR_PUBLIC_KEY", legacytuf.Rekor, "rekor.pub", tr, RekorKeysFromTrustedRoot)
	if err != nil {
		return nil, fmt.Errorf("getting Rekor public keys: %w", err)
	}
	material.CTLogPublicKeys, err = repositoryPublicKeys(client, "SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", legacytuf.CTFE, "ctfe.pub", tr, CTLogKeysFromTrustedRoot)
	if err != nil {
		return nil, fmt.Errorf("getting CTLog public keys: %w", err)
	}
	var targets []legacytuf.TargetFile
	if client != nil {
		targets, err = targetsFromClient(client, legacytuf.Fulcio, "fulcio.crt.pem", "fulcio_v1.crt.pem", "fulcio_intermediate_v1.crt.pem")
		if err != nil {
			return nil, fmt.Errorf("getting Fulcio targets: %w", err)
		}
	}
	material.FulcioRoots, material.FulcioIntermediates, err = fulcioRootsFromRepository(tr, targets, true)
	if err != nil {
		return nil, fmt.Errorf("getting Fulcio material: %w", err)
	}
	return material, nil
}

func repositoryPublicKeys(client *legacyTUFClient, environment string, usage legacytuf.UsageKind, name string, tr *root.TrustedRoot, fallback func(*root.TrustedRoot) (*cosign.TrustedTransparencyLogPubKeys, error)) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if file := os.Getenv(environment); file != "" {
		return publicKeysFromFile(file)
	} else if client != nil {
		targets, err := targetsFromClient(client, usage, name)
		if err != nil {
			return nil, err
		}
		if len(targets) != 0 {
			return publicKeysFromTargets(targets)
		}
	}
	return fallback(tr)
}
