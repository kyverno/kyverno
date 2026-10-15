package sigstoreguard

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// A valid local repository permits testing legacy trust formats without making
// external requests. Its blocked mirror also detects accidental cache misses.
func TestLegacyTargetsAndPolicyCacheIsolation(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv("TUF_ROOT", cacheRoot)
	t.Setenv("SIGSTORE_NO_CACHE", "false")
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	mirror := "http://169.254.169.254"
	cacheDir := filepath.Join(cacheRoot, tuf.URLToPath(mirror))
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "targets"), 0o700))

	pub, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := signature.LoadSignerVerifier(key, crypto.Hash(0))
	require.NoError(t, err)
	tufKey, err := metadata.KeyFromPublicKey(pub)
	require.NoError(t, err)
	expires := time.Now().Add(time.Hour)
	rootMetadata := metadata.Root(expires)
	for _, role := range []string{metadata.ROOT, metadata.TARGETS, metadata.SNAPSHOT, metadata.TIMESTAMP} {
		require.NoError(t, rootMetadata.Signed.AddKey(tufKey, role))
	}
	publicKey, err := cryptoutils.MarshalPublicKeyToPEM(pub)
	require.NoError(t, err)
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "legacy-fulcio"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: expires,
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, pub, key)
	require.NoError(t, err)
	targets := metadata.Targets(expires)
	for name, data := range map[string][]byte{
		"rekor.pub":      publicKey,
		"ctfe.pub":       publicKey,
		"fulcio.crt.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}),
	} {
		target, err := metadata.TargetFile().FromBytes(name, data, "sha256")
		require.NoError(t, err)
		targets.Signed.Targets[name] = target
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "targets", name), data, 0o600))
	}
	type signedMetadata interface {
		Sign(signature.Signer) (*metadata.Signature, error)
		ToBytes(bool) ([]byte, error)
	}
	var rootBytes []byte
	for name, document := range map[string]signedMetadata{
		"root.json":      rootMetadata,
		"targets.json":   targets,
		"snapshot.json":  metadata.Snapshot(expires),
		"timestamp.json": metadata.Timestamp(expires),
	} {
		_, err := document.Sign(signer)
		require.NoError(t, err)
		data, err := document.ToBytes(false)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, name), data, 0o600))
		if name == "root.json" {
			rootBytes = data
		}
	}

	mu.Lock()
	oldMirror, oldRoot := defaultMirror, defaultRoot
	defaultMirror, defaultRoot = mirror, rootBytes
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defaultMirror, defaultRoot = oldMirror, oldRoot
		mu.Unlock()
	})

	// The controller can still use legacy mirrors without trusted_root.json.
	rekor, err := RekorPublicKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, rekor.Keys, 1)
	ctlog, err := CTLogPublicKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, ctlog.Keys, 1)
	roots, intermediates, err := FulcioRoots()
	require.NoError(t, err)
	require.NotNil(t, roots)
	require.NotNil(t, intermediates)

	// Discover rotated keys by authenticated usage tags, retaining their status
	// instead of silently falling back to only rekor.pub/ctfe.pub.
	rotatedPub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	rotatedPEM, err := cryptoutils.MarshalPublicKeyToPEM(rotatedPub)
	require.NoError(t, err)
	for name, item := range map[string]struct {
		data   []byte
		usage  string
		status string
	}{
		"rekor-2025.pub":  {publicKey, "Rekor", "Expired"},
		"rekor-2026.pub":  {rotatedPEM, "Rekor", "Active"},
		"ctfe-2026.pub":   {rotatedPEM, "CTFE", "Active"},
		"fulcio-2025.pem": {pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}), "Fulcio", "Expired"},
	} {
		target, err := metadata.TargetFile().FromBytes(name, item.data, "sha256")
		require.NoError(t, err)
		custom, err := json.Marshal(map[string]any{"sigstore": map[string]string{"usage": item.usage, "status": item.status}})
		require.NoError(t, err)
		raw := json.RawMessage(custom)
		target.Custom = &raw
		targets.Signed.Targets[name] = target
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "targets", name), item.data, 0o600))
	}
	targets.ClearSignatures()
	_, err = targets.Sign(signer)
	require.NoError(t, err)
	targetMetadata, err := targets.ToBytes(false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "targets.json"), targetMetadata, 0o600))
	rekor, err = RekorPublicKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, rekor.Keys, 2)
	statuses := map[legacytuf.StatusKind]int{}
	for _, key := range rekor.Keys {
		statuses[key.Status]++
	}
	require.Equal(t, 1, statuses[legacytuf.Active])
	require.Equal(t, 1, statuses[legacytuf.Expired])
	ctlog, err = CTLogPublicKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, ctlog.Keys, 1)
	for _, key := range ctlog.Keys {
		require.Equal(t, rotatedPub, key.PubKey)
	}
	roots, _, err = FulcioRoots()
	require.NoError(t, err)
	require.NotNil(t, roots)

	// A policy cannot borrow or alter that valid cache when selecting a mirror
	// and trust anchor, even if they match the controller configuration.
	_, err = TrustedRootFor(context.Background(), mirror, rootBytes)
	require.ErrorIs(t, err, egress.ErrAddressBlocked)
	cachedRoot, err := os.ReadFile(filepath.Join(cacheDir, "root.json"))
	require.NoError(t, err)
	require.Equal(t, rootBytes, cachedRoot)
	mu.Lock()
	configuredMirror, configuredRoot := defaultMirror, append([]byte(nil), defaultRoot...)
	mu.Unlock()
	require.Equal(t, mirror, configuredMirror)
	require.Equal(t, rootBytes, configuredRoot)

	// Operator file mirrors still load signed legacy targets, while policies
	// cannot use the local fetcher to read the operator's filesystem.
	localDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(localDir, "targets"), 0o700))
	for from, to := range map[string]string{"root.json": "1.root.json", "timestamp.json": "timestamp.json", "snapshot.json": "1.snapshot.json", "targets.json": "1.targets.json"} {
		data, err := os.ReadFile(filepath.Join(cacheDir, from))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(localDir, to), data, 0o600))
	}
	for name, info := range targets.Signed.Targets {
		data, err := os.ReadFile(filepath.Join(cacheDir, "targets", name))
		require.NoError(t, err)
		file := hex.EncodeToString(info.Hashes["sha256"]) + "." + name
		require.NoError(t, os.WriteFile(filepath.Join(localDir, "targets", file), data, 0o600))
	}
	fileMirror := "file://" + filepath.ToSlash(localDir)
	require.NoError(t, Initialize(context.Background(), fileMirror, rootBytes))
	rekor, err = RekorPublicKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, rekor.Keys, 2)
	_, err = TrustedRootFor(context.Background(), fileMirror, rootBytes)
	require.ErrorContains(t, err, "HTTP or HTTPS")

}
