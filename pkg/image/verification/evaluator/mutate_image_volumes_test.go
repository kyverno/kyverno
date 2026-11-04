package evaluator

import (
	"context"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MutateDigest builds its patches from the JSON pointers pkg/utils/api extracts, so an image
// volume referenced by tag is pinned at its own reference, next to the containers. Volumes
// without an image, references that already carry a digest and references the policy does
// not match are left alone.
func TestMutateDigestPinsImageVolumes(t *testing.T) {
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
	pinned := "example.com/model@sha256:" + strings.Repeat("b", 64)
	require.NoError(t, unstructured.SetNestedSlice(pod.Object, []any{
		map[string]any{"name": "model", "image": map[string]any{"reference": "example.com/model:v1"}},
		map[string]any{"name": "config", "configMap": map[string]any{"name": "cm"}},
		map[string]any{"name": "pinned", "image": map[string]any{"reference": pinned}},
		map[string]any{"name": "elsewhere", "image": map[string]any{"reference": "other.example/model:v1"}},
	}, "spec", "volumes"))

	patches, err := compiled.MutateDigest(
		context.Background(), mutationImages{}, nil, nil,
		attr, request, nil, pod, nil,
		config.NewDefaultConfiguration(false), nil,
	)
	require.NoError(t, err)
	digest := "sha256:" + strings.Repeat("a", 64)
	got := map[string]any{}
	for _, patch := range patches {
		require.Equal(t, "replace", patch.Operation)
		got[patch.Path] = patch.Value
	}
	require.Equal(t, map[string]any{
		"/spec/containers/0/image":        "example.com/app:tag@" + digest,
		"/spec/volumes/0/image/reference": "example.com/model:v1@" + digest,
	}, got)
}
