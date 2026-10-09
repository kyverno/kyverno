package manifest

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/cosign/v3/pkg/oci/signed"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	manifestutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/sigstore/sigstore/pkg/tuf"
	"github.com/stretchr/testify/require"
)

func TestManifestImageSelectsMatchingVerifiedSigner(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(registry.New())
	t.Cleanup(server.Close)
	policy, err := egress.New(egress.Config{})
	require.NoError(t, err)
	// Keep the guarded request path, but route its synthetic public address to
	// an in-process registry. The test does not depend on DNS or a live registry.
	transport := policy.WrapTransport(&http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "192.0.2.1:80" {
			return nil, fmt.Errorf("unexpected registry address %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}})
	opts := []gcrremote.Option{gcrremote.WithTransport(transport)}
	ref, err := name.NewTag("192.0.2.1/manifests:test", name.Insecure)
	require.NoError(t, err)
	image := signed.Image(empty.Image)
	digest, err := image.Digest()
	require.NoError(t, err)
	message, err := (payload.Cosign{Image: ref.Context().Digest(digest.String())}).MarshalJSON()
	require.NoError(t, err)
	rootCerts := x509.NewCertPool()
	logKeys := cosign.NewTrustedTransparencyLogPubKeys()
	for _, signer := range []string{"other@example.com", manifestBundleSigner} {
		fixture := newManifestBundleFixtureWithSigner(t, false, signer)
		fixture.message = message
		hash := sha256.Sum256(message)
		fixture.sig, err = ecdsa.SignASN1(rand.Reader, fixture.leafKey, hash[:])
		require.NoError(t, err)
		require.True(t, rootCerts.AppendCertsFromPEM(fixture.rootPEM))
		require.NoError(t, logKeys.AddTransparencyLogPubKey(fixture.rekorPEM, tuf.Active))
		sig, err := static.NewSignature(message, base64.StdEncoding.EncodeToString(fixture.sig),
			static.WithCertChain(fixture.certPEM, nil), static.WithBundle(fixture.bundle(t, fixture.sig)))
		require.NoError(t, err)
		image, err = mutate.AttachSignatureToImage(image, sig)
		require.NoError(t, err)
	}
	require.NoError(t, gcrremote.Write(ref, image, opts...))
	registryOpts := []ociremote.Option{ociremote.WithRemoteOptions(opts...), ociremote.WithNameOptions(name.Insecure)}
	require.NoError(t, ociremote.WriteSignatures(ref.Context(), image, registryOpts...))
	co := &cosign.CheckOpts{
		RegistryClientOpts: registryOpts, ClaimVerifier: cosign.SimpleClaimVerifier,
		RootCerts: rootCerts, RekorPubKeys: &logKeys, IgnoreSCT: true, Offline: true,
		Identities: []cosign.Identity{{Issuer: manifestBundleIssuer}}, MaxWorkers: 1,
	}
	// Check that both signatures pass real certificate, payload and bundle
	// verification and that the allowed signer is deliberately second.
	verified, _, err := cosign.VerifyImageSignatures(context.Background(), ref, co)
	require.NoError(t, err)
	require.Len(t, verified, 2)
	cert, err := verified[0].Cert()
	require.NoError(t, err)
	require.Equal(t, "other@example.com", manifestutil.GetNameInfoFromCert(cert))
	for _, test := range []struct {
		name       string
		key        string
		signers    k8smanifest.SignerList
		wantSigner string
		wantMatch  bool
	}{
		{name: "matching signer is second", signers: k8smanifest.SignerList{manifestBundleSigner}, wantSigner: manifestBundleSigner, wantMatch: true},
		{name: "matching signer is first", signers: k8smanifest.SignerList{"other@example.com"}, wantSigner: "other@example.com", wantMatch: true},
		{name: "unrestricted", wantSigner: "other@example.com", wantMatch: true},
		{name: "all identities mismatch", signers: k8smanifest.SignerList{"absent@example.com"}, wantSigner: "other@example.com"},
		{name: "keyless wildcard retains exact identity check", signers: k8smanifest.SignerList{"manifest-*"}, wantSigner: "other@example.com"},
		{name: "keyed wildcard can select second signer", key: "configured-key", signers: k8smanifest.SignerList{"manifest-*"}, wantSigner: manifestBundleSigner, wantMatch: true},
		{name: "keyed middle wildcard remains unsupported", key: "configured-key", signers: k8smanifest.SignerList{"manifest-*@example.com"}, wantSigner: "other@example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			vo := &k8smanifest.VerifyResourceOption{}
			vo.Signers = test.signers
			signer, err := verifyImageSignatures(context.Background(), ref, co, test.key, vo)
			require.NoError(t, err)
			require.Equal(t, test.wantSigner, signer)
			require.Equal(t, test.wantMatch, matchesSigner(vo, test.key, signer) && vo.Signers.Match(signer))
		})
	}
}
