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
	"github.com/sigstore/sigstore-go/pkg/tuf"
	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
)

// VerificationMaterial contains the trust used for both legacy signatures and
// modern bundles. All repository material comes from the same TUF trust anchor.
type VerificationMaterial struct {
	TrustedRoot         *root.TrustedRoot
	RekorPublicKeys     *cosign.TrustedTransparencyLogPubKeys
	CTLogPublicKeys     *cosign.TrustedTransparencyLogPubKeys
	FulcioRoots         *x509.CertPool
	FulcioIntermediates *x509.CertPool
}

// VerificationMaterialFor preserves operator key-file overrides and legacy PEM
// targets, falling back to trusted_root.json when individual targets are absent.
// Policy mirrors and roots cannot read or overwrite the operator's TUF cache.
func VerificationMaterialFor(ctx context.Context, mirror string, rootBytes []byte) (*VerificationMaterial, error) {
	return verificationMaterialFor(ctx, mirror, rootBytes, registryclient.EgressHTTPClient())
}

func verificationMaterialFor(ctx context.Context, mirror string, rootBytes []byte, httpClient *http.Client) (*VerificationMaterial, error) {
	mu.Lock()
	defer mu.Unlock()
	isolate := mirror != "" || len(rootBytes) != 0
	if !isolate {
		mirror, rootBytes = defaultMirror, defaultRoot
	}
	opts, err := tufOptions(ctx, mirror, rootBytes, true, httpClient, isolate)
	if err != nil {
		return nil, err
	}
	client, err := tuf.New(opts)
	if err != nil {
		return nil, fmt.Errorf("initializing TUF: %w", err)
	}
	data, err := client.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("getting trusted_root.json from TUF: %w", err)
	}
	tr, err := root.NewTrustedRootFromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parsing trusted root: %w", err)
	}
	// A separate go-tuf/v2 updater exposes authenticated usage tags for rotated
	// legacy targets. Both clients use the identical guarded fetcher and anchor.
	legacy, _ := newUpdaterClient(opts)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	material, err := verificationMaterialFromTargets(tr, legacy)
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	return material, err
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
	if client != nil {
		targets, targetErr := targetsFromClient(client, legacytuf.Fulcio, "fulcio.crt.pem", "fulcio_v1.crt.pem", "fulcio_intermediate_v1.crt.pem")
		if targetErr == nil {
			material.FulcioRoots, material.FulcioIntermediates, err = fulcioRootsFromTargetFiles(targets)
			if err == nil {
				return material, nil
			}
		}
	}
	material.FulcioRoots, material.FulcioIntermediates, err = FulcioRootsFromTrustedRoot(tr)
	if err != nil {
		return nil, fmt.Errorf("getting Fulcio material: %w", err)
	}
	return material, nil
}

func repositoryPublicKeys(client *legacyTUFClient, environment string, usage legacytuf.UsageKind, name string, tr *root.TrustedRoot, fallback func(*root.TrustedRoot) (*cosign.TrustedTransparencyLogPubKeys, error)) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if file := os.Getenv(environment); file != "" {
		if keys, err := publicKeysFromFile(file); err == nil {
			return keys, nil
		}
	} else if client != nil {
		if targets, err := targetsFromClient(client, usage, name); err == nil {
			if keys, err := publicKeysFromTargets(targets); err == nil {
				return keys, nil
			}
		}
	}
	return fallback(tr)
}
