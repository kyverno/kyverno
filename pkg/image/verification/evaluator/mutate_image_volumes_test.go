package evaluator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// imageVolumeDigest is the digest mutationImages resolves every reference to.
var imageVolumeDigest = "sha256:" + strings.Repeat("a", 64)

// failingImages resolves every reference to imageVolumeDigest except fail, which errors.
type failingImages struct {
	imagedataloader.ImageContext
	fail string
}

func (f failingImages) Get(ctx context.Context, image string, remoteOpts []remote.Option, nameOpts []name.Option) (*imagedataloader.ImageData, error) {
	if image == f.fail {
		return nil, errors.New("registry unreachable")
	}
	return mutationImages{}.Get(ctx, image, remoteOpts, nameOpts)
}

// mutateImageVolumePod runs MutateDigest for a policy matching example.com/* on a pod with a
// tagged container, a tagged image volume, a volume with no image, a volume already pinned by
// digest and one outside the policy's glob, and returns the patches keyed by path.
func mutateImageVolumePod(t *testing.T, images imagedataloader.ImageContext) (map[string]any, error) {
	t.Helper()
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-mutate-image-volumes"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "example.com/*"}},
		},
	}
	compiled, errs := NewCompiler(nil).Compile(policy, nil)
	require.Empty(t, errs)
	request, attr, pod := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	require.NoError(t, unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "main", "image": "example.com/app:tag"},
	}, "spec", "containers"))
	require.NoError(t, unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "model", "image": map[string]any{"reference": "example.com/model:v1"}},
		map[string]any{"name": "config", "configMap": map[string]any{"name": "cm"}},
		map[string]any{"name": "pinned", "image": map[string]any{"reference": "example.com/model@sha256:" + strings.Repeat("b", 64)}},
		map[string]any{"name": "elsewhere", "image": map[string]any{"reference": "other.example/model:v1"}},
	}, "spec", "volumes"))
	patches, err := compiled.MutateDigest(
		context.Background(), images, nil, nil,
		attr, request, nil, pod, nil,
		config.NewDefaultConfiguration(false), nil,
	)
	got := map[string]any{}
	for _, patch := range patches {
		require.Equal(t, "replace", patch.Operation)
		got[patch.Path] = patch.Value
	}
	return got, err
}

// MutateDigest builds its patches from the JSON pointers pkg/utils/api extracts, so an image
// volume referenced by tag is pinned at its own reference, next to the containers. Volumes
// without an image, references that already carry a digest and references the policy does
// not match are left alone.
func TestMutateDigestPinsImageVolumes(t *testing.T) {
	got, err := mutateImageVolumePod(t, mutationImages{})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"/spec/containers/0/image":        "example.com/app:tag@" + imageVolumeDigest,
		"/spec/volumes/0/image/reference": "example.com/model:v1@" + imageVolumeDigest,
	}, got)
}

// An image volume whose digest cannot be resolved is reported, and does not cost the
// container its patch.
func TestMutateDigestImageVolumeResolutionFailure(t *testing.T) {
	got, err := mutateImageVolumePod(t, failingImages{fail: "example.com/model:v1"})
	require.ErrorContains(t, err, "failed to resolve digest for image example.com/model:v1")
	require.Equal(t, map[string]any{
		"/spec/containers/0/image": "example.com/app:tag@" + imageVolumeDigest,
	}, got)
}
