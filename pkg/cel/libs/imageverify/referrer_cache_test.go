package imageverify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const sbomType = "sbom/cyclone-dx"

// fakeNotary verifies only referrers whose payload is {"signed":true}.
type fakeNotary struct{ calls int }

func (*fakeNotary) VerifyImageSignature(context.Context, *imagedataloader.ImageData, string, string) error {
	return errors.New("not implemented")
}

func (n *fakeNotary) VerifyAttestationSignature(_ context.Context, img *imagedataloader.ImageData, referrerType, _, _ string) ([]gcrv1.Descriptor, error) {
	n.calls++
	referrers, err := img.FetchReferrers(referrerType)
	if err != nil {
		return nil, err
	}
	for _, r := range referrers {
		if data, _, err := img.FetchReferrerData(r); err == nil && string(data) == `{"signed":true}` {
			img.AddVerifiedReferrer(r)
			return []gcrv1.Descriptor{r}, nil
		}
	}
	return nil, errors.New("no signed referrer")
}

type referrerTest struct {
	repo      name.Repository
	subject   gcrv1.Descriptor
	image     string
	hidden    atomic.Pointer[gcrv1.Hash] // left out of the referrers listing, still served by digest
	notary    *fakeNotary
	ivCache   imageverifycache.Client
	base      *IvFuncs
	attestors []v1beta1.Attestor
}

func newReferrerTest(t *testing.T) *referrerTest {
	t.Helper()
	rt := &referrerTest{notary: &fakeNotary{}}
	reg := registry.New(registry.WithReferrersSupport(true))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hidden := rt.hidden.Load()
		if hidden == nil || !strings.Contains(r.URL.Path, "/referrers/") {
			reg.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		reg.ServeHTTP(rec, r)
		var index gcrv1.IndexManifest
		if err := json.Unmarshal(rec.Body.Bytes(), &index); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		index.Manifests = slices.DeleteFunc(index.Manifests, func(d gcrv1.Descriptor) bool { return d.Digest == *hidden })
		w.Header().Set("Content-Type", rec.Header().Get("Content-Type"))
		_ = json.NewEncoder(w).Encode(index)
	}))
	t.Cleanup(srv.Close)

	var err error
	rt.repo, err = name.NewRepository(strings.TrimPrefix(srv.URL, "http://") + "/app")
	require.NoError(t, err)
	subject, err := random.Image(64, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(rt.repo.Tag("v1"), subject))
	desc, err := partial.Descriptor(subject)
	require.NoError(t, err)
	rt.subject, rt.image = *desc, rt.repo.Digest(desc.Digest.String()).String()

	rt.ivCache, err = imageverifycache.New(imageverifycache.WithCacheEnableFlag(true), imageverifycache.WithMaxSize(0), imageverifycache.WithTTLDuration(0))
	require.NoError(t, err)
	rt.attestors = []v1beta1.Attestor{{Name: "notary", Notary: &v1beta1.Notary{Certs: &v1beta1.StringOrExpression{Value: "cert"}}}}
	pol := &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "referrer-cache", UID: "referrer-cache", ResourceVersion: "1"},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestors:    rt.attestors,
			Attestations: []v1beta1.Attestation{{Name: "sbom", Referrer: &v1beta1.Referrer{Type: sbomType}}},
		},
	}
	rt.base = &IvFuncs{Adapter: types.DefaultTypeAdapter, logger: logr.Discard(), policy: pol, attestationList: attestationMap(pol), notaryVerifier: rt.notary}
	return rt
}

// push attaches an SBOM referrer with the given payload to the subject image.
func (rt *referrerTest) push(t *testing.T, payload string) gcrv1.Hash {
	t.Helper()
	img, err := mutate.AppendLayers(empty.Image, static.NewLayer([]byte(payload), ggcrtypes.MediaType("application/json")))
	require.NoError(t, err)
	img = mutate.ConfigMediaType(mutate.MediaType(img, ggcrtypes.OCIManifestSchema1), sbomType)
	referrer, ok := mutate.Subject(img, rt.subject).(gcrv1.Image)
	require.True(t, ok)
	digest, err := referrer.Digest()
	require.NoError(t, err)
	require.NoError(t, remote.Write(rt.repo.Digest(digest.String()), referrer))
	return digest
}

// admit evaluates verifyAttestationSignatures() then extractPayload() as a new
// admission request would, with nothing verified on its ImageData yet.
func (rt *referrerTest) admit(t *testing.T) (count ref.Val, payload func() ref.Val) {
	t.Helper()
	imgCtx, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(t, err)
	f := NewRuntimeForPolicy(context.Background(), rt.base, imgCtx, rt.ivCache, NewImageVerificationResults()).functions
	count = f.verify_image_attestations_string_string_stringarray(f.NativeToValue(rt.image), f.NativeToValue("sbom"), f.NativeToValue(rt.attestors))
	return count, func() ref.Val { return f.payload_string_string(f.NativeToValue(rt.image), f.NativeToValue("sbom")) }
}

func Test_impl_verify_referrer_attestation_cache_hit_restores_verified_referrer(t *testing.T) {
	t.Parallel()
	rt := newReferrerTest(t)
	signed := rt.push(t, `{"signed":true}`)

	count, payload := rt.admit(t)
	require.Equal(t, int64(1), count.Value())
	assert.Equal(t, map[string]any{"signed": true}, payload().Value())

	// The registry now lists only an unsigned referrer of the same type.
	rt.push(t, `{"signed":false}`)
	rt.hidden.Store(&signed)

	count, payload = rt.admit(t)
	require.Equal(t, int64(1), count.Value())
	assert.Equal(t, 1, rt.notary.calls, "the second admission must be a cache hit")
	assert.Equal(t, map[string]any{"signed": true}, payload().Value())
	assert.Equal(t, map[string]any{"signed": true}, payload().Value())
}

func Test_impl_verify_referrer_attestation_degraded_cache_entry_reverifies(t *testing.T) {
	t.Parallel()
	tests := map[string]map[string][]byte{
		"presence only":             nil,
		"not json":                  {sbomType: []byte(`not json`)},
		"empty list":                {sbomType: []byte(`[]`)},
		"descriptor without digest": {sbomType: []byte(`[{"artifactType":"sbom/cyclone-dx"}]`)},
		"descriptor of other type":  {sbomType: []byte(`[{"digest":"sha256:` + strings.Repeat("a", 64) + `","artifactType":"other"}]`)},
	}
	for name, cached := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rt := newReferrerTest(t)
			rt.push(t, `{"signed":true}`)
			rule, err := attestorCacheRule(attestationCacheRule, "sbom", rt.attestors)
			require.NoError(t, err)
			stored, err := rt.ivCache.SetWithPayload(context.TODO(), rt.base.policy, rule, rt.image, true, cached)
			require.NoError(t, err)
			require.True(t, stored)

			count, payload := rt.admit(t)
			require.Equal(t, int64(1), count.Value())
			assert.Equal(t, 1, rt.notary.calls, "a degraded entry must not be a cache hit")
			assert.Equal(t, map[string]any{"signed": true}, payload().Value())
		})
	}
}

func Test_impl_verify_referrer_attestation_failure_is_not_cached(t *testing.T) {
	t.Parallel()
	rt := newReferrerTest(t)
	rt.push(t, `{"signed":false}`)

	for range 2 {
		count, _ := rt.admit(t)
		require.Equal(t, int64(0), count.Value())
	}
	assert.Equal(t, 2, rt.notary.calls, "a failed verification must not be cached")
}

func Test_verifiedReferrersPayload_dedupes_and_keeps_only_restore_fields(t *testing.T) {
	t.Parallel()
	digest := gcrv1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}
	verified := gcrv1.Descriptor{
		MediaType:    ggcrtypes.OCIManifestSchema1,
		Digest:       digest,
		Size:         42,
		ArtifactType: sbomType,
		Annotations:  map[string]string{"large": strings.Repeat("x", 1024)},
		Data:         []byte(strings.Repeat("x", 1024)),
	}
	attest := v1beta1.Attestation{Name: "sbom", Referrer: &v1beta1.Referrer{Type: sbomType}}

	// Two Notary attestors that verified the same referrer.
	payloads := verifiedReferrersPayload(attest, []gcrv1.Descriptor{verified, verified})
	descs, ok := decodeVerifiedReferrers(payloads[sbomType], sbomType)
	require.True(t, ok)
	assert.Equal(t, []gcrv1.Descriptor{{MediaType: ggcrtypes.OCIManifestSchema1, Digest: digest, Size: 42, ArtifactType: sbomType}}, descs)
}
