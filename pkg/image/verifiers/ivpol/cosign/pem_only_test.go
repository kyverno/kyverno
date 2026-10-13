package cosign

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/sigstoreguard"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestCheckOptionsPEMOnlyOperatorRepository(t *testing.T) {
	// Operator defaults are process-wide. Run the real startup and IVPOL path
	// in a child so this fixture cannot alter other tests' trust configuration.
	const child = "KYVERNO_TEST_PEM_ONLY_OPERATOR"
	if os.Getenv(child) != "true" {
		command := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestCheckOptionsPEMOnlyOperatorRepository$", "-test.timeout=1m")
		command.Env = append(os.Environ(), child+"=true")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}
	t.Setenv("TUF_ROOT", t.TempDir())
	t.Setenv("SIGSTORE_NO_CACHE", "false")
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	rootBytes, mirror, certificate := pemOnlyOperatorRepository(t)
	require.NoError(t, sigstoreguard.Initialize(context.Background(), mirror, rootBytes))
	attestor := &v1beta1.Cosign{Keyless: &v1beta1.Keyless{Identities: []v1beta1.Identity{{Issuer: testIssuer, Subject: testSubject}}}}
	remoteOptions, nameOptions := baseOpts()
	opts, err := checkOptions(context.Background(), attestor, remoteOptions, nameOptions, nil)
	require.NoError(t, err)
	require.Len(t, opts.RekorPubKeys.Keys, 1)
	require.Len(t, opts.CTLogPubKeys.Keys, 1)
	require.True(t, opts.Offline)
	_, err = certificate.Verify(x509.VerifyOptions{Roots: opts.RootCerts})
	require.NoError(t, err, "legacy verification must use the operator's authenticated Fulcio root")
	require.True(t, opts.TrustedMaterial == nil, "a PEM-only repository must not provide a typed nil bundle trust interface")
	require.NoError(t, validateBundleTrust(opts), "legacy signatures retain their authenticated PEM trust")
	opts.NewBundleFormat = true
	require.ErrorContains(t, validateBundleTrust(opts), "modern Sigstore bundle verification requires trusted root material")
	testBundleDetectionWithPEMOnlyTrust(t)
}

func pemOnlyOperatorRepository(t *testing.T) ([]byte, string, *x509.Certificate) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "targets"), 0o700))
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := signature.LoadSignerVerifier(key, crypto.Hash(0))
	require.NoError(t, err)
	tufKey, err := metadata.KeyFromPublicKey(pub)
	require.NoError(t, err)
	expires := time.Now().Add(time.Hour)
	root := metadata.Root(expires)
	for _, role := range []string{metadata.ROOT, metadata.TARGETS, metadata.SNAPSHOT, metadata.TIMESTAMP} {
		require.NoError(t, root.Signed.AddKey(tufKey, role))
	}
	publicKey, err := cryptoutils.MarshalPublicKeyToPEM(pub)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "operator-fulcio"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: expires, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	targets := metadata.Targets(expires)
	for name, data := range map[string][]byte{"rekor.pub": publicKey, "ctfe.pub": publicKey, "fulcio.crt.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})} {
		target, err := metadata.TargetFile().FromBytes(name, data, "sha256")
		require.NoError(t, err)
		targets.Signed.Targets[name] = target
		require.NoError(t, os.WriteFile(filepath.Join(dir, "targets", hex.EncodeToString(target.Hashes["sha256"])+"."+name), data, 0o600))
	}
	type signedMetadata interface {
		Sign(signature.Signer) (*metadata.Signature, error)
		ToBytes(bool) ([]byte, error)
	}
	var rootBytes []byte
	for name, document := range map[string]signedMetadata{"1.root.json": root, "1.targets.json": targets, "1.snapshot.json": metadata.Snapshot(expires), "timestamp.json": metadata.Timestamp(expires)} {
		_, err := document.Sign(signer)
		require.NoError(t, err)
		data, err := document.ToBytes(false)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
		if name == "1.root.json" {
			rootBytes = data
		}
	}
	return rootBytes, (&url.URL{Scheme: "file", Path: dir}).String(), certificate
}
