package sigstoreguard

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore/pkg/tuf"
)

// RekorKeysFromTrustedRoot extracts Rekor transparency log public keys from
// a TrustedRoot, used as a fallback when individual TUF targets (rekor.pub)
// are unavailable.
func RekorKeysFromTrustedRoot(tr *root.TrustedRoot) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if tr == nil {
		return nil, fmt.Errorf("trusted root not available")
	}
	keys := cosign.NewTrustedTransparencyLogPubKeys()
	added := 0
	var lastErr error
	for _, tlog := range tr.RekorLogs() {
		pemBytes, err := publicKeyToPEM(tlog.PublicKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to encode Rekor public key: %w", err)
			continue
		}
		if err := keys.AddTransparencyLogPubKey(pemBytes, logKeyStatus(tlog.ValidityPeriodStart, tlog.ValidityPeriodEnd)); err != nil {
			lastErr = fmt.Errorf("failed to add Rekor public key: %w", err)
			continue
		}
		added++
	}
	if added == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no Rekor public keys found in trusted root")
	}
	return &keys, nil
}

// CTLogKeysFromTrustedRoot extracts CT log public keys from a TrustedRoot,
// used as a fallback when individual TUF targets (ctfe.pub) are unavailable.
func CTLogKeysFromTrustedRoot(tr *root.TrustedRoot) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if tr == nil {
		return nil, fmt.Errorf("trusted root not available")
	}
	keys := cosign.NewTrustedTransparencyLogPubKeys()
	added := 0
	var lastErr error
	for _, tlog := range tr.CTLogs() {
		pemBytes, err := publicKeyToPEM(tlog.PublicKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to encode CTLog public key: %w", err)
			continue
		}
		if err := keys.AddTransparencyLogPubKey(pemBytes, logKeyStatus(tlog.ValidityPeriodStart, tlog.ValidityPeriodEnd)); err != nil {
			lastErr = fmt.Errorf("failed to add CTLog public key: %w", err)
			continue
		}
		added++
	}
	if added == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no CTLog public keys found in trusted root")
	}
	return &keys, nil
}

// FulcioRootsFromTrustedRoot extracts Fulcio root and intermediate
// certificates from a TrustedRoot, used as a fallback when individual
// TUF targets (fulcio_v1.crt.pem) are unavailable.
func FulcioRootsFromTrustedRoot(tr *root.TrustedRoot) (*x509.CertPool, *x509.CertPool, error) {
	return fulcioRootsFromTrustedRoot(tr, true)
}

func fulcioRootsFromTrustedRoot(tr *root.TrustedRoot, activeOnly bool) (*x509.CertPool, *x509.CertPool, error) {
	if tr == nil {
		return nil, nil, fmt.Errorf("trusted root not available")
	}
	cas := tr.FulcioCertificateAuthorities()
	if len(cas) == 0 {
		return nil, nil, fmt.Errorf("no certificate authority in trusted root")
	}
	roots := x509.NewCertPool()
	intermediates := x509.NewCertPool()
	rootsAdded := 0
	for _, ca := range cas {
		fca, ok := ca.(*root.FulcioCertificateAuthority)
		if !ok {
			continue
		}
		if activeOnly && logKeyStatus(fca.ValidityPeriodStart, fca.ValidityPeriodEnd) == tuf.Expired {
			continue
		}
		if fca.Root != nil {
			roots.AddCert(fca.Root)
			rootsAdded++
		}
		for _, inter := range fca.Intermediates {
			if inter != nil {
				intermediates.AddCert(inter)
			}
		}
	}
	if rootsAdded == 0 {
		return nil, nil, fmt.Errorf("no Fulcio root certificates found in trusted root")
	}
	return roots, intermediates, nil
}

func publicKeyToPEM(pubKey interface{}) ([]byte, error) {
	derBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derBytes,
	}), nil
}

func logKeyStatus(start, end time.Time) tuf.StatusKind {
	now := time.Now()
	if !start.IsZero() && now.Before(start) {
		return tuf.Expired
	}
	if !end.IsZero() && now.After(end) {
		return tuf.Expired
	}
	return tuf.Active
}
