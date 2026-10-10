package cosign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	cosignattestation "github.com/sigstore/cosign/v3/pkg/cosign/attestation"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/dsse"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/require"
)

// signLegacy signs digest the way cosign v2 does without a transparency log: a
// simple-signing .sig signature and an in-toto .att attestation of the custom
// predicate type. It returns the PEM public key that verifies both.
func signLegacy(t *testing.T, digest name.Digest) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	signer, err := signature.LoadECDSASignerVerifier(key, crypto.SHA256)
	require.NoError(t, err)

	se, err := ociremote.SignedEntity(digest)
	require.NoError(t, err)

	simple, err := payload.Cosign{Image: digest}.MarshalJSON()
	require.NoError(t, err)
	raw, err := signer.SignMessage(bytes.NewReader(simple))
	require.NoError(t, err)
	sig, err := static.NewSignature(simple, base64.StdEncoding.EncodeToString(raw))
	require.NoError(t, err)
	se, err = mutate.AttachSignatureToEntity(se, sig)
	require.NoError(t, err)

	statement := fmt.Sprintf(`{"_type":"https://in-toto.io/Statement/v1","subject":[{"name":%q,"digest":{"sha256":%q}}],"predicateType":%q,"predicate":{}}`,
		digest.Repository.String(), strings.TrimPrefix(digest.DigestStr(), "sha256:"), cosignattestation.CosignCustomProvenanceV01)
	envelope, err := dsse.WrapSigner(signer, "application/vnd.in-toto+json").SignMessage(strings.NewReader(statement))
	require.NoError(t, err)
	att, err := static.NewAttestation(envelope)
	require.NoError(t, err)
	se, err = mutate.AttachAttestationToEntity(se, att)
	require.NoError(t, err)

	require.NoError(t, ociremote.WriteSignatures(digest.Repository, se))
	require.NoError(t, ociremote.WriteAttestations(digest.Repository, se))

	pub, err := cryptoutils.MarshalPublicKeyToPEM(&key.PublicKey)
	require.NoError(t, err)
	return string(pub)
}

// pushImageWithBrokenBundle pushes an image carrying a referrer whose manifest
// names a sigstore bundle layer but whose blob is not a bundle: detection, which
// reads no blobs, counts it, while cosign.GetBundles cannot load it.
func pushImageWithBrokenBundle(t *testing.T, host string) (string, name.Digest) {
	t.Helper()
	image := fmt.Sprintf("%s/test/image:tag", host)
	ref, err := name.ParseReference(image, name.Insecure)
	require.NoError(t, err)
	pushed, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, pushed))
	digest, err := pushed.Digest()
	require.NoError(t, err)
	subject := ref.Context().Digest(digest.String())
	attachReferrer(t, subject, "", bundleMediaTypePrefix+".v0.3+json", "not a bundle")
	return image, subject
}

// TestNoValidBundlesMatchesCosign pins the error cosign.GetBundles returns when
// no referrer holds a bundle it can load, which is what the legacy fallback keys
// on. If a cosign upgrade changes it, this fails rather than the fallback quietly
// stopping.
func TestNoValidBundlesMatchesCosign(t *testing.T) {
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	defer server.Close()
	_, subject := pushImageWithBrokenBundle(t, strings.TrimPrefix(server.URL, "http://"))

	_, _, err := cosign.GetBundles(context.Background(), subject, nil)
	require.True(t, noValidBundles(err), "GetBundles error not recognised: %v", err)
	require.True(t, noValidBundles(fmt.Errorf("wrapped: %w", err)))
	require.False(t, noValidBundles(nil))
	require.False(t, noValidBundles(errors.New("no valid bundles exist in registry")), "only cosign's error type counts")
}

// TestBrokenBundleFallsBackToLegacy guards that a referrer detection counts as a
// bundle, but cosign cannot load, does not decide the outcome. cosign.GetBundles,
// which detection replaced, found no valid bundle on such an image and left
// verification on the legacy path, so a valid legacy signature still verified.
func TestBrokenBundleFallsBackToLegacy(t *testing.T) {
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	defer server.Close()
	image, subject := pushImageWithBrokenBundle(t, strings.TrimPrefix(server.URL, "http://"))
	publicKey := signLegacy(t, subject)

	idf, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	img, err := idf.FetchImageData(context.Background(), image, nil, []name.Option{name.Insecure})
	require.NoError(t, err)
	detected, err := hasSigstoreBundles(img, &cosign.CheckOpts{RegistryClientOpts: []ociremote.Option{ociremote.WithRemoteOptions(img.RemoteOpts()...)}})
	require.NoError(t, err)
	require.True(t, detected, "detection must count the broken bundle, otherwise this test proves nothing")

	attestor := &v1beta1.Attestor{Name: "legacy", Cosign: &v1beta1.Cosign{
		Key:   &v1beta1.Key{Data: publicKey},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true, InsecureIgnoreSCT: true},
	}}
	verifier := NewVerifier(nil, logr.Discard())

	t.Run("signature", func(t *testing.T) {
		require.NoError(t, verifier.VerifyImageSignature(context.Background(), img, attestor))
	})
	t.Run("attestation", func(t *testing.T) {
		// the legacy path accepts an attestation only with a transparency log
		// entry, which this test cannot produce, so its verdict here is that
		// rejection -- the same one main returns for this image -- rather than the
		// broken bundle's "no valid bundles exist in registry"
		attestation := &v1beta1.Attestation{Name: "custom", InToto: &v1beta1.InToto{Type: "custom"}}
		require.EqualError(t, verifier.VerifyAttestationSignature(context.Background(), img, attestation, attestor), "cosign bundle verification failed")
	})
}
