package cosign

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const emptyConfigMediaType = "application/vnd.oci.empty.v1+json"

// countingRegistry records the path of every request so a test can assert what
// bundle detection actually asks the registry for.
type countingRegistry struct {
	inner http.Handler
	mu    sync.Mutex
	paths []string
}

func (c *countingRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.paths = append(c.paths, r.URL.Path)
	c.mu.Unlock()
	c.inner.ServeHTTP(w, r)
}

func (c *countingRegistry) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths = nil
}

func (c *countingRegistry) count(substr string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, p := range c.paths {
		if strings.Contains(p, substr) {
			n++
		}
	}
	return n
}

// rawManifest lets a test put a hand-built manifest, controlling both its
// artifactType, which go-containerregistry copies into the fallback index entry
// (or the config media type when it is empty), and its layer media type.
type rawManifest []byte

func (r rawManifest) RawManifest() ([]byte, error)        { return r, nil }
func (r rawManifest) MediaType() (types.MediaType, error) { return types.OCIManifestSchema1, nil }

// attachReferrer attaches a referrer to subject carrying one layer, shaped the
// way cosign shapes a bundle referrer: an empty config, the bundle as the only
// layer, and the image as the subject. An empty artifactType reproduces the index
// entry that says nothing about its contents (kyverno#16664).
func attachReferrer(t *testing.T, subject name.Digest, artifactType, layerMediaType, payload string, opts ...remote.Option) {
	t.Helper()
	attachReferrerIn(t, subject.Repository, subject, artifactType, layerMediaType, payload, opts...)
}

// attachReferrerIn is attachReferrer writing the referrer into repo rather than
// the subject's own repository, as cosign does for an attestor that configures a
// signature repository.
func attachReferrerIn(t *testing.T, repo name.Repository, subject name.Digest, artifactType, layerMediaType, payload string, opts ...remote.Option) {
	t.Helper()
	layer := static.NewLayer([]byte(payload), types.MediaType(layerMediaType))
	require.NoError(t, remote.WriteLayer(repo, layer, opts...))
	layerDigest, err := layer.Digest()
	require.NoError(t, err)
	layerSize, err := layer.Size()
	require.NoError(t, err)

	config := static.NewLayer([]byte("{}"), types.MediaType(emptyConfigMediaType))
	require.NoError(t, remote.WriteLayer(repo, config, opts...))
	configDigest, err := config.Digest()
	require.NoError(t, err)
	configSize, err := config.Size()
	require.NoError(t, err)

	subjectDesc, err := remote.Head(subject, opts...)
	require.NoError(t, err)

	manifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config:        v1.Descriptor{MediaType: types.MediaType(emptyConfigMediaType), Digest: configDigest, Size: configSize},
		Layers:        []v1.Descriptor{{MediaType: types.MediaType(layerMediaType), Digest: layerDigest, Size: layerSize}},
		Subject:       &v1.Descriptor{MediaType: subjectDesc.MediaType, Digest: subjectDesc.Digest, Size: subjectDesc.Size},
		ArtifactType:  artifactType,
	}
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	digest, _, err := v1.SHA256(strings.NewReader(string(body)))
	require.NoError(t, err)
	require.NoError(t, remote.Put(repo.Digest(digest.String()), rawManifest(body), opts...))
}

// TestHasSigstoreBundlesReadsNoBlobs is the regression test for #17833: bundle
// format detection must not download the bundles. cosign.GetBundles, which this
// replaced, fetched and parsed the blob behind every referrer only to decide a
// boolean, and VerifyImageAttestations then fetched them all again to verify.
func TestHasSigstoreBundlesReadsNoBlobs(t *testing.T) {
	tests := []struct {
		name string
		// artifactType is what the referrers entry advertises, if anything
		artifactType   string
		layerMediaType string
		attach         bool
		want           bool
		wantManifests  int
	}{{
		// an entry typed with the bundle media type, as a referrers-API registry
		// reports what cosign writes
		name:           "detected from a bundle-typed entry",
		artifactType:   bundleMediaTypePrefix + ".v0.3+json",
		layerMediaType: bundleMediaTypePrefix + ".v0.3+json",
		attach:         true,
		want:           true,
		wantManifests:  1,
	}, {
		// the artifactType says bundle but the layer is not one, as a tool that
		// sets only the artifactType writes it: ociremote.Bundle rejects it, so
		// GetBundles would not have counted it and verification must stay on the
		// legacy path
		name:           "a bundle-typed entry without a bundle layer is not new-format",
		artifactType:   bundleMediaTypePrefix + ".v0.3+json",
		layerMediaType: "application/vnd.oci.image.layer.v1.tar",
		attach:         true,
		want:           false,
		wantManifests:  1,
	}, {
		// kyverno#16664: entries typed from an empty config say nothing, so the
		// referrer manifest has to be read -- but still not the bundle blob
		name:           "detected by reading the referrer manifest",
		layerMediaType: bundleMediaTypePrefix + ".v0.3+json",
		attach:         true,
		want:           true,
		wantManifests:  1,
	}, {
		// ociremote.Bundle rejects pre-v0.3 bundles, so GetBundles would not
		// have counted this one and verification must stay on the legacy path
		name:           "a pre-v0.3 bundle is not new-format",
		layerMediaType: bundleMediaTypePrefix + "+json;version=0.2",
		attach:         true,
		want:           false,
		wantManifests:  1,
	}, {
		name:   "no referrers at all",
		attach: false,
		want:   false,
	}, {
		name:           "a referrer that is not a bundle",
		layerMediaType: "application/vnd.in-toto+json",
		attach:         true,
		want:           false,
		wantManifests:  1,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &countingRegistry{inner: registry.New(registry.Logger(log.New(io.Discard, "", 0)))}
			server := httptest.NewServer(handler)
			defer server.Close()
			host := strings.TrimPrefix(server.URL, "http://")

			image := fmt.Sprintf("%s/test/image:tag", host)
			ref, err := name.ParseReference(image, name.Insecure)
			require.NoError(t, err)
			pushed, err := random.Image(256, 1)
			require.NoError(t, err)
			require.NoError(t, remote.Write(ref, pushed))

			digest, err := pushed.Digest()
			require.NoError(t, err)
			if tt.attach {
				attachReferrer(t, ref.Context().Digest(digest.String()), tt.artifactType, tt.layerMediaType, `{"mediaType":"test-bundle"}`)
			}

			idf, err := imagedataloader.New(nil, nil, nil)
			require.NoError(t, err)
			img, err := idf.FetchImageData(context.Background(), image, nil, []name.Option{name.Insecure})
			require.NoError(t, err)

			cOpts := &cosign.CheckOpts{RegistryClientOpts: []ociremote.Option{
				ociremote.WithRemoteOptions(img.RemoteOpts()...),
				ociremote.WithNameOptions(img.NameOpts()...),
			}}

			// count only what detection itself asks for
			handler.reset()
			got, err := hasSigstoreBundles(img, cOpts)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)

			require.Zero(t, handler.count("/blobs/"), "detection must never download a bundle blob")
			// the index is read once, from the referrers API or, as here, the
			// sha256-<digest> fallback tag go-containerregistry drops back to
			require.Equal(t, 1, handler.count("/referrers/"))
			require.Equal(t, 1, handler.count("/manifests/sha256-"))
			// referrer manifests are addressed by digest
			require.Equal(t, tt.wantManifests, handler.count("/manifests/sha256:"))
		})
	}
}

// TestIsBundleMediaType pins the version rule that keeps detection in agreement
// with ociremote.Bundle, which parses the blob and rejects anything below v0.3.
func TestIsBundleMediaType(t *testing.T) {
	for mediaType, want := range map[string]bool{
		bundleMediaTypePrefix + ".v0.3+json":        true,
		bundleMediaTypePrefix + ".v0.4+json":        true,
		bundleMediaTypePrefix + "+json;version=0.3": true,
		bundleMediaTypePrefix + "+json;version=0.2": false,
		bundleMediaTypePrefix + "+json;version=0.1": false,
		emptyConfigMediaType:                        false,
		"application/vnd.in-toto+json":              false,
		"":                                          false,
	} {
		require.Equal(t, want, isBundleMediaType(mediaType), "media type %q", mediaType)
	}
}

// TestDetectionDoesNotRefetchBundles compares what detection costs against what
// cosign.GetBundles cost, on the same image. GetBundles is still what performs
// the verification afterwards (VerifyImageAttestations calls it), so every blob
// it reads during detection is read a second time to verify: that is the
// duplicate download #17833 reports.
func TestDetectionDoesNotRefetchBundles(t *testing.T) {
	handler := &countingRegistry{inner: registry.New(registry.Logger(log.New(io.Discard, "", 0)))}
	server := httptest.NewServer(handler)
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")

	image := fmt.Sprintf("%s/test/image:tag", host)
	ref, err := name.ParseReference(image, name.Insecure)
	require.NoError(t, err)
	pushed, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, pushed))
	digest, err := pushed.Digest()
	require.NoError(t, err)

	// three bundles, as an image carrying a signature, an SBOM and a
	// vulnerability report would
	for i := range 3 {
		attachReferrer(t, ref.Context().Digest(digest.String()), "", bundleMediaTypePrefix+".v0.3+json",
			fmt.Sprintf(`{"mediaType":"test-bundle","n":%d}`, i))
	}

	idf, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	img, err := idf.FetchImageData(context.Background(), image, nil, []name.Option{name.Insecure})
	require.NoError(t, err)
	cOpts := &cosign.CheckOpts{RegistryClientOpts: []ociremote.Option{
		ociremote.WithRemoteOptions(img.RemoteOpts()...),
		ociremote.WithNameOptions(img.NameOpts()...),
	}}

	handler.reset()
	_, _, _ = cosign.GetBundles(context.Background(), img.NameRef(), cOpts.RegistryClientOpts)
	wasBlobs, wasManifests := handler.count("/blobs/"), handler.count("/manifests/sha256:")

	handler.reset()
	detected, err := hasSigstoreBundles(img, cOpts)
	require.NoError(t, err)
	require.True(t, detected)
	nowBlobs, nowManifests := handler.count("/blobs/"), handler.count("/manifests/sha256:")

	t.Logf("discovery cost: GetBundles %d blobs / %d manifests, detection %d blobs / %d manifests",
		wasBlobs, wasManifests, nowBlobs, nowManifests)

	require.Positive(t, wasBlobs, "GetBundles downloads the bundles, otherwise this test proves nothing")
	require.Zero(t, nowBlobs, "detection must not download any bundle")
	require.Less(t, nowManifests, wasManifests, "detection must read fewer manifests than GetBundles")
}

// TestDetectionAgreesWithGetBundlesOverPlainHTTP pins that detection reads the
// referrer manifests the way GetBundles does when verification calls it. kyverno
// calls VerifyImageAttestations without name options, so GetBundles parses each
// referrer without name.Insecure and reads it over HTTPS. Were detection to keep
// name.Insecure, it would find bundles on a plain-HTTP registry that verification
// then cannot read, and fail an admission the legacy path would have handled.
func TestDetectionAgreesWithGetBundlesOverPlainHTTP(t *testing.T) {
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	defer server.Close()
	// a hostname, so go-containerregistry picks HTTP only when told to
	const host = "registry.test:5000"
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "http" {
			return nil, fmt.Errorf("plain-HTTP registry: %s %s", r.URL.Scheme, r.URL.Host)
		}
		r = r.Clone(r.Context())
		r.URL.Host = strings.TrimPrefix(server.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	opts := []remote.Option{remote.WithTransport(transport)}

	image := fmt.Sprintf("%s/test/image:tag", host)
	ref, err := name.ParseReference(image, name.Insecure)
	require.NoError(t, err)
	pushed, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, pushed, opts...))
	digest, err := pushed.Digest()
	require.NoError(t, err)
	// typed as a bundle in the index, which detection must not take on trust
	attachReferrer(t, ref.Context().Digest(digest.String()), bundleMediaTypePrefix+".v0.3+json", bundleMediaTypePrefix+".v0.3+json",
		`{"mediaType":"test-bundle"}`, opts...)

	idf, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	img, err := idf.FetchImageData(context.Background(), image, opts, []name.Option{name.Insecure})
	require.NoError(t, err)
	cOpts := &cosign.CheckOpts{RegistryClientOpts: []ociremote.Option{
		ociremote.WithRemoteOptions(img.RemoteOpts()...),
		ociremote.WithNameOptions(img.NameOpts()...),
	}}

	bundles, _, _ := cosign.GetBundles(context.Background(), img.NameRef(), cOpts.RegistryClientOpts)
	detected, err := hasSigstoreBundles(img, cOpts)
	require.NoError(t, err)
	require.Empty(t, bundles, "GetBundles reads the referrer over HTTPS, otherwise this test proves nothing")
	require.False(t, detected, "detection must not find bundles that verification cannot read")
}

// TestDetectionReadsTheSignatureRepository pins that detection looks for bundles
// where verification will: in the attestor's signature repository, reached with
// the credentials its signaturePullSecrets name. The options come from
// checkOptions, as in production, and the image repository carries no
// referrers, so only a read of the signature repository can find the bundle.
func TestDetectionReadsTheSignatureRepository(t *testing.T) {
	var requireAuth atomic.Bool
	inner := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// the image repository is open; the signature repository only accepts
		// the credentials of its pull secret
		if user, password, ok := r.BasicAuth(); requireAuth.Load() && strings.HasPrefix(r.URL.Path, "/v2/signatures/") &&
			(!ok || user != "sig" || password != "pw") {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")

	image := fmt.Sprintf("%s/test/image:tag", host)
	ref, err := name.ParseReference(image, name.Insecure)
	require.NoError(t, err)
	pushed, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, pushed))
	digest, err := pushed.Digest()
	require.NoError(t, err)
	signatures, err := name.NewRepository(host+"/signatures", name.Insecure)
	require.NoError(t, err)
	attachReferrerIn(t, signatures, ref.Context().Digest(digest.String()), "", bundleMediaTypePrefix+".v0.3+json", `{"mediaType":"test-bundle"}`)
	requireAuth.Store(true)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	require.NoError(t, indexer.Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "signatures", Namespace: config.KyvernoNamespace()},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`,
			host, base64.StdEncoding.EncodeToString([]byte("sig:pw"))))},
	}))

	idf, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	img, err := idf.FetchImageData(context.Background(), image, nil, []name.Option{name.Insecure})
	require.NoError(t, err)

	tests := []struct {
		name        string
		pullSecrets []corev1.LocalObjectReference
		want        bool
		wantErr     bool
	}{{
		name:        "found in the signature repository with its pull secret",
		pullSecrets: []corev1.LocalObjectReference{{Name: "signatures"}},
		want:        true,
	}, {
		// the index read is rejected, so detection reports an error and
		// verification stays on the legacy path, as it did when GetBundles failed
		name:    "without the pull secret the signature repository is not read",
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cosignCfg := &v1beta1.Cosign{
				Key:    &v1beta1.Key{Data: testPublicKey},
				CTLog:  &v1beta1.CTLog{InsecureIgnoreTlog: true},
				Source: &v1beta1.Source{Repository: signatures.String(), SignaturePullSecrets: tt.pullSecrets},
			}
			cOpts, err := checkOptions(context.Background(), cosignCfg, img.RemoteOpts(), img.NameOpts(), corev1listers.NewSecretLister(indexer))
			require.NoError(t, err)

			got, err := hasSigstoreBundles(img, cOpts)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
