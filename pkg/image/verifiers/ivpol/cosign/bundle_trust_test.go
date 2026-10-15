package cosign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
)

// Called from the isolated PEM-only operator test to exercise real bundle
// discovery while retaining its authenticated legacy trust configuration.
func testBundleDetectionWithPEMOnlyTrust(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	server := httptest.NewServer(registry.New(registry.WithReferrersSupport(true)))
	t.Cleanup(server.Close)
	ref, err := name.ParseReference(strings.TrimPrefix(server.URL, "http://")+"/image:latest", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, empty.Image))
	fetcher, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	image, err := fetcher.FetchImageData(ctx, ref.String(), nil, []name.Option{name.Insecure})
	require.NoError(t, err)
	verifier := NewVerifier(nil, logr.Discard())
	keyless := &v1beta1.Cosign{Keyless: &v1beta1.Keyless{Identities: []v1beta1.Identity{{Issuer: testIssuer, Subject: testSubject}}}}
	opts, _, _, err := verifier.buildCheckOptsWithBundleDetection(ctx, keyless, image)
	require.NoError(t, err, "legacy image verification retains operator PEM trust")
	require.False(t, opts.NewBundleFormat)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	signer, err := signature.LoadECDSASignerVerifier(key, crypto.SHA256)
	require.NoError(t, err)
	publicKey, err := cryptoutils.MarshalPublicKeyToPEM(key.Public())
	require.NoError(t, err)
	artifact := []byte("offline bundle payload")
	digest := sha256.Sum256(artifact)
	sig, err := signer.SignMessage(bytes.NewReader(artifact))
	require.NoError(t, err)
	bundleJSON := []byte(fmt.Sprintf(`{"mediaType":"application/vnd.dev.sigstore.bundle+json;version=0.3","verificationMaterial":{"publicKey":{"hint":""}},"messageSignature":{"messageDigest":{"algorithm":"SHA2_256","digest":%q},"signature":%q}}`, base64.StdEncoding.EncodeToString(digest[:]), base64.StdEncoding.EncodeToString(sig)))
	imageDigest := ref.Context().Digest(image.Digest)
	require.NoError(t, ociremote.WriteAttestationNewBundleFormat(imageDigest, bundleJSON, "https://cosign.sigstore.dev/attestation/v1"))
	bundles, _, err := cosign.GetBundles(ctx, ref, nil)
	require.NoError(t, err)
	require.Len(t, bundles, 1, "the fixture must publish a discoverable modern bundle")

	for _, test := range []struct {
		name     string
		attestor *v1beta1.Cosign
	}{
		{"keyless", keyless},
		{"key requiring transparency log", &v1beta1.Cosign{Key: &v1beta1.Key{Data: string(publicKey)}}},
		{"key requiring timestamp", &v1beta1.Cosign{Key: &v1beta1.Key{Data: string(publicKey)}, CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true, TSACertChain: testTSACertChain}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts, _, _, err := verifier.buildCheckOptsWithBundleDetection(ctx, test.attestor, image)
			require.ErrorContains(t, err, "modern Sigstore bundle verification requires trusted root material")
			require.Nil(t, opts)
		})
	}

	t.Run("offline key bundle", func(t *testing.T) {
		attestor := &v1beta1.Cosign{Key: &v1beta1.Key{Data: string(publicKey)}, CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true}}
		opts, _, _, err := verifier.buildCheckOptsWithBundleDetection(ctx, attestor, image)
		require.NoError(t, err)
		require.True(t, opts.NewBundleFormat)
		require.True(t, opts.TrustedMaterial == nil)
		result, err := cosign.VerifyNewBundle(ctx, opts, verify.WithArtifact(bytes.NewReader(artifact)), bundles[0])
		require.NoError(t, err, "a genuine offline key bundle must remain verifiable without modern TUF material")
		require.NotNil(t, result)
	})
}
