package sigstoreguard

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore/pkg/signature"
	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestVerificationMaterialHonorsOperatorKeyOverrides(t *testing.T) {
	fixture := newVerificationFixture(t)
	for _, omitLogs := range []bool{false, true} {
		t.Run(fmt.Sprintf("trusted_root_logs_omitted=%t", omitLogs), func(t *testing.T) {
			tr := fixture.trustedRoot
			if omitLogs {
				var err error
				tr, err = root.NewTrustedRoot(root.TrustedRootMediaType01, tr.FulcioCertificateAuthorities(), nil, nil, nil)
				require.NoError(t, err)
			}
			file := filepath.Join(t.TempDir(), "operator.pub")
			require.NoError(t, os.WriteFile(file, fixture.legacyPEM, 0o600))
			t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", file)
			t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", file)
			material, err := verificationMaterialFromTargets(tr, nil)
			require.NoError(t, err)
			for _, keys := range []map[string]legacytuf.StatusKind{keyStatuses(material.RekorPublicKeys.Keys), keyStatuses(material.CTLogPublicKeys.Keys)} {
				require.Equal(t, map[string]legacytuf.StatusKind{fixture.legacyID: legacytuf.Active}, keys)
			}
		})
	}
	// Invalid overrides retain the previous trusted_root fallback behavior.
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", filepath.Join(t.TempDir(), "missing"))
	material, err := verificationMaterialFromTargets(fixture.trustedRoot, nil)
	require.NoError(t, err)
	require.Contains(t, material.RekorPublicKeys.Keys, fixture.modernID)
	require.Contains(t, material.CTLogPublicKeys.Keys, fixture.modernID)
}

func TestVerificationMaterialPreservesLegacyAndModernTrust(t *testing.T) {
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	t.Setenv("TUF_ROOT", t.TempDir())
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy_targets=%t", legacy), func(t *testing.T) {
			fixture := newVerificationFixture(t)
			rootBytes, mirror, client := fixture.repository(t, legacy)
			// Alternate policies must never fall back to the operator's default.
			mu.Lock()
			previousMirror, previousRoot := defaultMirror, defaultRoot
			defaultMirror, defaultRoot = "http://169.254.169.254", []byte("different operator root")
			mu.Unlock()
			t.Cleanup(func() {
				mu.Lock()
				defaultMirror, defaultRoot = previousMirror, previousRoot
				mu.Unlock()
			})
			material, err := verificationMaterialFor(context.Background(), mirror, rootBytes, client)
			require.NoError(t, err)
			require.NotNil(t, material.TrustedRoot)
			if legacy {
				// Rotated usage-tagged PEM files take precedence over different
				// modern keys and preserve the authenticated log-key status.
				require.Equal(t, map[string]legacytuf.StatusKind{fixture.legacyID: legacytuf.Expired}, keyStatuses(material.RekorPublicKeys.Keys))
				require.Equal(t, map[string]legacytuf.StatusKind{fixture.legacyID: legacytuf.Active}, keyStatuses(material.CTLogPublicKeys.Keys))
				_, err = fixture.historicalLeaf.Verify(x509.VerifyOptions{
					Roots: material.FulcioRoots, Intermediates: material.FulcioIntermediates,
					CurrentTime: fixture.signingTime, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
				})
				require.NoError(t, err, "historical signatures must retain their legacy Fulcio CA")
			} else {
				require.Equal(t, map[string]legacytuf.StatusKind{fixture.modernID: legacytuf.Active}, keyStatuses(material.RekorPublicKeys.Keys))
				require.Equal(t, map[string]legacytuf.StatusKind{fixture.modernID: legacytuf.Active}, keyStatuses(material.CTLogPublicKeys.Keys))
				_, err = fixture.modernCA.Verify(x509.VerifyOptions{Roots: material.FulcioRoots})
				require.NoError(t, err)
			}
			// Explicit policy roots must not create or read a shared disk cache.
			_, err = os.Stat(filepath.Join(os.Getenv("TUF_ROOT"), tuf.URLToPath(mirror)))
			require.ErrorIs(t, err, os.ErrNotExist)
			// A different root cannot reuse trust from the previous policy.
			other := newVerificationFixture(t)
			otherRoot, _, _ := other.repository(t, false)
			_, err = verificationMaterialFor(context.Background(), mirror, otherRoot, client)
			require.Error(t, err)
		})
	}
}

func TestVerificationMaterialRejectsBlockedMirrorAndCancellation(t *testing.T) {
	_, err := VerificationMaterialFor(context.Background(), "http://169.254.169.254", nil)
	require.ErrorIs(t, err, egress.ErrAddressBlocked)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = VerificationMaterialFor(ctx, "", nil)
	require.ErrorIs(t, err, context.Canceled)
}

type verificationFixture struct {
	trustedRoot    *root.TrustedRoot
	modernCA       *x509.Certificate
	historicalCA   *x509.Certificate
	historicalLeaf *x509.Certificate
	signingTime    time.Time
	modernID       string
	legacyID       string
	legacyPEM      []byte
	key            *ecdsa.PrivateKey
}

func newVerificationFixture(t *testing.T) *verificationFixture {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now().Truncate(time.Second)
	newCA := func(name string, start, end time.Time) *x509.Certificate {
		template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: name}, NotBefore: start, NotAfter: end, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		require.NoError(t, err)
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		return cert
	}
	modern := newCA("modern-fulcio", now.Add(-time.Hour), now.Add(time.Hour))
	historical := newCA("historical-fulcio", now.Add(-72*time.Hour), now.Add(-24*time.Hour))
	leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "historical-signer"}, NotBefore: now.Add(-60 * time.Hour), NotAfter: now.Add(-36 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}
	der, err := x509.CreateCertificate(rand.Reader, leafTemplate, historical, &key.PublicKey, key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	modernDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	modernID := sha256.Sum256(modernDER)
	log := &root.TransparencyLog{BaseURL: "https://logs.example", ID: modernID[:], ValidityPeriodStart: now.Add(-time.Hour), ValidityPeriodEnd: now.Add(time.Hour), HashFunc: crypto.SHA256, PublicKey: &key.PublicKey, SignatureHashFunc: crypto.SHA256}
	logs := map[string]*root.TransparencyLog{hex.EncodeToString(modernID[:]): log}
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, []root.CertificateAuthority{&root.FulcioCertificateAuthority{Root: modern, ValidityPeriodStart: modern.NotBefore, ValidityPeriodEnd: modern.NotAfter}}, logs, nil, logs)
	require.NoError(t, err)
	legacyKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	legacyDER, err := x509.MarshalPKIXPublicKey(&legacyKey.PublicKey)
	require.NoError(t, err)
	legacyID := sha256.Sum256(legacyDER)
	return &verificationFixture{trustedRoot: tr, modernCA: modern, historicalCA: historical, historicalLeaf: leaf, signingTime: now.Add(-48 * time.Hour), modernID: hex.EncodeToString(modernID[:]), legacyID: hex.EncodeToString(legacyID[:]), legacyPEM: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: legacyDER}), key: key}
}

func (f *verificationFixture) repository(t *testing.T, includeLegacy bool, extra ...legacytuf.TargetFile) ([]byte, string, *http.Client) {
	t.Helper()
	expires := time.Now().Add(time.Hour)
	signer, err := signature.LoadSignerVerifier(f.key, crypto.SHA256)
	require.NoError(t, err)
	tufKey, err := metadata.KeyFromPublicKey(&f.key.PublicKey)
	require.NoError(t, err)
	rootMetadata := metadata.Root(expires)
	for _, role := range []string{metadata.ROOT, metadata.TARGETS, metadata.SNAPSHOT, metadata.TIMESTAMP} {
		require.NoError(t, rootMetadata.Signed.AddKey(tufKey, role))
	}
	targets := metadata.Targets(expires)
	files := map[string][]byte{}
	addTarget := func(name string, data []byte, usage, status string) {
		target, err := metadata.TargetFile().FromBytes(name, data, "sha256")
		require.NoError(t, err)
		if usage != "" {
			custom, err := json.Marshal(map[string]any{"sigstore": map[string]string{"usage": usage, "status": status}})
			require.NoError(t, err)
			raw := json.RawMessage(custom)
			target.Custom = &raw
		}
		targets.Signed.Targets[name] = target
		files["/targets/"+hex.EncodeToString(target.Hashes["sha256"])+"."+name] = data
	}
	trJSON, err := f.trustedRoot.MarshalJSON()
	require.NoError(t, err)
	addTarget("trusted_root.json", trJSON, "", "")
	if includeLegacy {
		addTarget("rekor-previous.pub", f.legacyPEM, "Rekor", "Expired")
		addTarget("ctfe-previous.pub", f.legacyPEM, "CTFE", "Active")
		addTarget("fulcio-previous.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.historicalCA.Raw}), "Fulcio", "Expired")
	}
	for _, target := range extra {
		addTarget(target.Name, target.Target, "Fulcio", "Active")
	}
	type signedMetadata interface {
		Sign(signature.Signer) (*metadata.Signature, error)
		ToBytes(bool) ([]byte, error)
	}
	for path, document := range map[string]signedMetadata{"/1.root.json": rootMetadata, "/1.targets.json": targets, "/1.snapshot.json": metadata.Snapshot(expires), "/timestamp.json": metadata.Timestamp(expires)} {
		_, err := document.Sign(signer)
		require.NoError(t, err)
		files[path], err = document.ToBytes(false)
		require.NoError(t, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data, ok := files[r.URL.Path]; ok {
			_, _ = w.Write(data)
		} else {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	policy, err := egress.New(egress.Config{})
	require.NoError(t, err)
	transport := policy.WrapTransport(&http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "93.184.216.34:80" {
			return nil, fmt.Errorf("unexpected destination %q", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}})
	t.Cleanup(transport.CloseIdleConnections)
	return files["/1.root.json"], "http://93.184.216.34", &http.Client{Transport: transport}
}

func keyStatuses(keys map[string]cosign.TransparencyLogPubKey) map[string]legacytuf.StatusKind {
	statuses := make(map[string]legacytuf.StatusKind, len(keys))
	for id, key := range keys {
		statuses[id] = key.Status
	}
	return statuses
}

func TestVerificationMaterialFallsBackFromIntermediateOnlyTargets(t *testing.T) {
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	fixture := newVerificationFixture(t)
	template := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "intermediate"}, NotBefore: fixture.modernCA.NotBefore, NotAfter: fixture.modernCA.NotAfter, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, fixture.modernCA, &fixture.key.PublicKey, fixture.key)
	require.NoError(t, err)
	rootBytes, mirror, client := fixture.repository(t, false, legacytuf.TargetFile{Name: "fulcio_intermediate_v1.crt.pem", Target: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})})
	material, err := verificationMaterialFor(context.Background(), mirror, rootBytes, client)
	require.NoError(t, err)
	_, err = fixture.modernCA.Verify(x509.VerifyOptions{Roots: material.FulcioRoots})
	require.NoError(t, err, "intermediate-only legacy targets must not hide trusted_root.json roots")
}
