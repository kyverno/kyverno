package cosign

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	ocistatic "github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBundleMediaType = "application/vnd.dev.sigstore.bundle.v0.3+json"
	testEmptyConfigType = "application/vnd.oci.empty.v1+json"

	// validBundle is the smallest content that cosign accepts as a v0.3 sigstore bundle.
	validBundle = `{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json","verificationMaterial":{"publicKey":{"hint":"x"}},"messageSignature":{"messageDigest":{"algorithm":"SHA2_256","digest":"YQ=="},"signature":"YQ=="}}`
)

type testTaggable struct {
	raw []byte
	mt  types.MediaType
}

func (t testTaggable) RawManifest() ([]byte, error)        { return t.raw, nil }
func (t testTaggable) MediaType() (types.MediaType, error) { return t.mt, nil }

// newBundleRegistry starts an in-process OCI registry with referrers support and
// counts blob GETs so a test can assert that detection never reads a layer.
func newBundleRegistry(t *testing.T) (name.Repository, *atomic.Int64) {
	t.Helper()
	var blobGets atomic.Int64
	inner := registry.New(registry.WithReferrersSupport(true))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blobs/sha256:") {
			blobGets.Add(1)
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	repo, err := name.NewRepository(strings.TrimPrefix(srv.URL, "http://") + "/test/img")
	require.NoError(t, err)
	return repo, &blobGets
}

// pushSubject writes a small random image under the given tag and returns its digest.
func pushSubject(t *testing.T, repo name.Repository, tag string) v1.Descriptor {
	t.Helper()
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(repo.Tag(tag), img))
	d, err := img.Digest()
	require.NoError(t, err)
	sz, err := img.Size()
	require.NoError(t, err)
	return v1.Descriptor{MediaType: types.OCIManifestSchema1, Digest: d, Size: sz}
}

// pushReferrer attaches a single-layer referrer manifest to subject.
func pushReferrer(t *testing.T, repo name.Repository, subject v1.Descriptor, layerData []byte, layerMediaType, artifactType string) {
	t.Helper()
	layer := static.NewLayer(layerData, types.MediaType(layerMediaType))
	require.NoError(t, remote.WriteLayer(repo, layer))
	layerDigest, err := layer.Digest()
	require.NoError(t, err)

	cfg := static.NewLayer([]byte("{}"), types.MediaType(testEmptyConfigType))
	require.NoError(t, remote.WriteLayer(repo, cfg))
	cfgDigest, err := cfg.Digest()
	require.NoError(t, err)

	m := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		ArtifactType:  artifactType,
		Config:        v1.Descriptor{MediaType: types.MediaType(testEmptyConfigType), Digest: cfgDigest, Size: 2},
		Layers: []v1.Descriptor{{
			MediaType: types.MediaType(layerMediaType),
			Digest:    layerDigest,
			Size:      int64(len(layerData)),
		}},
		Subject: &subject,
	}
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	h, _, err := v1.SHA256(bytes.NewReader(raw))
	require.NoError(t, err)
	require.NoError(t, remote.Put(repo.Digest(h.String()), testTaggable{raw: raw, mt: types.OCIManifestSchema1}))
}

// gzipPaddedBundle returns a valid v0.3 bundle padded with n bytes of JSON
// whitespace and gzip-compressed, so the blob is tiny but expands greatly.
func gzipPaddedBundle(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write([]byte(validBundle))
	require.NoError(t, err)
	pad := bytes.Repeat([]byte(" "), 1<<20)
	for written := 0; written < n; written += len(pad) {
		_, err = gw.Write(pad)
		require.NoError(t, err)
	}
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// keyAttestor returns an attestor that takes the no-Sigstore-infrastructure fast
// path in checkOptions, so these tests need no network beyond the test registry.
func bundleTestAttestor() *v1beta1.Cosign {
	return &v1beta1.Cosign{
		Key:   &v1beta1.Key{Data: testPublicKey},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
	}
}

func fetchTestImage(t *testing.T, ref string) *imagedataloader.ImageData {
	t.Helper()
	idf, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	img, err := idf.FetchImageData(context.TODO(), ref, nil, nil)
	require.NoError(t, err)
	return img
}

// Test_BundleDetection_OversizedBundleIsNotDetected covers a referrer whose blob is
// tiny but inflates far past the read limit: detection must give up and report no
// bundle rather than allocating the expansion or failing the request.
func Test_BundleDetection_OversizedBundleIsNotDetected(t *testing.T) {
	repo, blobGets := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	blob := gzipPaddedBundle(t, 64<<20)
	pushReferrer(t, repo, subject, blob, testBundleMediaType, testBundleMediaType)

	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())

	blobGets.Store(0)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	cOpts, _, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	runtime.ReadMemStats(&after)
	require.NoError(t, err)

	alloc := after.TotalAlloc - before.TotalAlloc
	t.Logf("referrer blob is %d bytes, inflates to >%d; detection allocated %d bytes over %d blob GETs", len(blob), 64<<20, alloc, blobGets.Load())

	assert.False(t, cOpts.NewBundleFormat, "a bundle that cannot be read within the limit is not a v3 bundle")
	// A bounded read is still one read; the point is that it stops at the limit.
	assert.Equal(t, int64(1), blobGets.Load(), "detection must read the candidate layer exactly once")
	assert.Less(t, alloc, uint64(64<<20), "detection must not allocate the inflated bundle")
}

func Test_BundleDetection_Positive(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, testBundleMediaType)

	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())
	cOpts, _, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)
	assert.True(t, cOpts.NewBundleFormat)
}

func Test_BundleDetection_NoArtifactTypeOnReferrerManifest(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	// A bundle referrer need not declare an artifactType, so detection cannot rely on it.
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, "")

	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())
	cOpts, _, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)
	assert.True(t, cOpts.NewBundleFormat)
}

func Test_BundleDetection_NegativeNoReferrers(t *testing.T) {
	repo, blobGets := newBundleRegistry(t)
	pushSubject(t, repo, "subject")

	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())
	blobGets.Store(0)
	cOpts, _, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)
	assert.False(t, cOpts.NewBundleFormat)
	assert.Zero(t, blobGets.Load())
}

func Test_BundleDetection_NegativeNonBundleReferrer(t *testing.T) {
	repo, blobGets := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(`{}`), "application/vnd.in-toto+json", "application/vnd.in-toto+json")

	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())
	blobGets.Store(0)
	cOpts, _, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)
	assert.False(t, cOpts.NewBundleFormat)
	assert.Zero(t, blobGets.Load())
}

// junkBundle has the bundle media type but content cosign cannot accept, which is
// what anyone able to push a referrer into the repository can plant.
const junkBundle = `{"not":"a bundle"}`

// signV2 attaches a cosign v2 simple-signing signature for the image at ref and
// returns the PEM public key an attestor needs to verify it.
func signV2(t *testing.T, repo name.Repository, tag string) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(priv.Public())
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	ref, err := name.ParseReference(repo.Tag(tag).String())
	require.NoError(t, err)
	se, err := ociremote.SignedEntity(ref)
	require.NoError(t, err)
	d, err := se.(interface{ Digest() (v1.Hash, error) }).Digest()
	require.NoError(t, err)

	payloadBytes, err := (&payload.Cosign{Image: repo.Digest(d.String())}).MarshalJSON()
	require.NoError(t, err)
	signer, err := signature.LoadECDSASignerVerifier(priv, crypto.SHA256)
	require.NoError(t, err)
	sig, err := signer.SignMessage(bytes.NewReader(payloadBytes))
	require.NoError(t, err)

	ociSig, err := ocistatic.NewSignature(payloadBytes, base64.StdEncoding.EncodeToString(sig))
	require.NoError(t, err)
	newSE, err := mutate.AttachSignatureToEntity(se, ociSig)
	require.NoError(t, err)
	require.NoError(t, ociremote.WriteSignatures(repo, newSE))
	return string(pubPEM)
}

func keyAttestorFor(pubPEM string) *v1beta1.Cosign {
	return &v1beta1.Cosign{
		Key:   &v1beta1.Key{Data: pubPEM},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
	}
}

// Test_V2SignedImage_WithPlantedBundleReferrer_IsAdmitted is the regression guard:
// a referrer with the bundle media type but unusable content must not divert a
// correctly v2-signed image onto the attestation path, which would deny it.
func Test_V2SignedImage_WithPlantedBundleReferrer_IsAdmitted(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pubPEM := signV2(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(junkBundle), testBundleMediaType, testBundleMediaType)

	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}

	cOpts, _, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), attestor.Cosign, img)
	require.NoError(t, err)
	assert.False(t, cOpts.NewBundleFormat, "an unparseable bundle referrer must not be detected as v3")

	require.NoError(t, v.VerifyImageSignature(context.TODO(), img, attestor),
		"a v2-signed image with a planted bundle referrer must be admitted")
}

// detectionEquivalence asserts loadBundles and cosign.GetBundles agree on a
// fixture, both on whether a bundle was found and, when both found one, on how
// many: a future divergence in cosign's acceptance predicate must fail a test
// rather than silently change behaviour.
func detectionEquivalence(t *testing.T, repo name.Repository, want bool) {
	t.Helper()
	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())
	cOpts, bundles, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)

	// GetBundles reports "none found" as ErrNoMatchingAttestations, not as success.
	cosignBundles, _, err := cosign.GetBundles(context.TODO(), img.NameRef(), cOpts.RegistryClientOpts)
	var noneFound *cosign.ErrNoMatchingAttestations
	if err != nil {
		require.ErrorAs(t, err, &noneFound)
	}
	cosignSays := err == nil && len(cosignBundles) > 0

	t.Logf("detector=%v (%d bundles) cosign.GetBundles=%v (%d bundles) (want %v)",
		cOpts.NewBundleFormat, len(bundles), cosignSays, len(cosignBundles), want)
	assert.Equal(t, want, cOpts.NewBundleFormat, "detector disagrees with expectation")
	assert.Equal(t, cosignSays, cOpts.NewBundleFormat, "detector disagrees with cosign.GetBundles")
	if cosignSays && cOpts.NewBundleFormat {
		assert.Equal(t, len(cosignBundles), len(bundles), "detector and cosign.GetBundles disagree on bundle count")
	}
}

func Test_BundleDetection_MatchesCosign_GenuineBundle(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, testBundleMediaType)
	detectionEquivalence(t, repo, true)
}

func Test_BundleDetection_MatchesCosign_JunkBundle(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(junkBundle), testBundleMediaType, testBundleMediaType)
	detectionEquivalence(t, repo, false)
}

func Test_BundleDetection_MatchesCosign_NoReferrers(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	pushSubject(t, repo, "subject")
	detectionEquivalence(t, repo, false)
}

func Test_BundleDetection_MatchesCosign_NonSigstoreReferrer(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(`{}`), "application/vnd.in-toto+json", "application/vnd.in-toto+json")
	detectionEquivalence(t, repo, false)
}

// Test_BundleDetection_MatchesCosign_GzippedBundle guards the only structural
// difference between the two readers: ReadLayerLimited sniffs the compression
// magic while cosign reads the layer uncompressed.
func Test_BundleDetection_MatchesCosign_GzippedBundle(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, gzipPaddedBundle(t, 0), testBundleMediaType, testBundleMediaType)
	detectionEquivalence(t, repo, true)
}

// Test_BundleDetection_MatchesCosign_TwoGenuineBundles makes the bundle-count
// comparison in detectionEquivalence load-bearing: a fixture with a single
// bundle cannot tell "found a bundle" apart from "found the right number".
func Test_BundleDetection_MatchesCosign_TwoGenuineBundles(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, testBundleMediaType)
	bundle2, _ := signV3Bundle(t, subject.Digest.Hex, "https://example.com/predicate")
	pushReferrer(t, repo, subject, bundle2, testBundleMediaType, testBundleMediaType)
	detectionEquivalence(t, repo, true)
}

// Test_BundleDetection_InsecureRegistryKeepsHTTP covers a plain-HTTP registry
// reached by a hostname that go-containerregistry does not treat as local
// (localhost, loopback and RFC 1918 literals default to HTTP anyway). With
// allowInsecureRegistry the image reference carries name.Insecure, and every
// referrer reference built from it must keep that flag, otherwise the referrer
// manifest is requested over HTTPS and the bundle is silently skipped.
func Test_BundleDetection_InsecureRegistryKeepsHTTP(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, testBundleMediaType)

	// Route every connection to the plain-HTTP test registry, whatever the host.
	addr := repo.RegistryStr()
	dialer := &net.Dialer{}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	ref, err := name.ParseReference("registry.kyverno.test:"+port+"/test/img:subject", name.Insecure)
	require.NoError(t, err)
	require.Equal(t, "http", ref.Context().Scheme())

	v := NewVerifier(nil, logr.Discard())
	bundles, hash := v.loadBundles(ref, []ociremote.Option{
		ociremote.WithRemoteOptions(remote.WithTransport(transport)),
	})
	require.NotNil(t, hash, "the subject digest must resolve over HTTP")
	assert.Len(t, bundles, 1, "the bundle referrer must be fetched over HTTP, like the referrers list")
}
