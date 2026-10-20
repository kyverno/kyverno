package compiler

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

// countingImageContext counts image fetches that reach the libs context.
type countingImageContext struct {
	*libs.FakeContextProvider
	calls atomic.Int64
}

func (c *countingImageContext) GetImageData(image string, opts []remote.Option) (map[string]any, error) {
	c.calls.Add(1)
	return c.FakeContextProvider.GetImageData(image, opts)
}

// allowedBaseImages is the expression of the kyverno/policies allowed-base-images policy,
// which calls image.GetMetadata four times per container.
const allowedBaseImagesExpression = `object.spec.containers.all(container, has(image.GetMetadata(container.image).manifest.annotations) && image.GetMetadata(container.image).manifest.annotations != null && 'org.opencontainers.image.base.name' in image.GetMetadata(container.image).manifest.annotations && image.GetMetadata(container.image).manifest.annotations['org.opencontainers.image.base.name'] in variables.allowedBaseImages)`

func TestEvaluate_ImageMetadataFetchedOncePerImagePerEvaluation(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "allowed-base-images"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{{
				Name:       "allowedBaseImages",
				Expression: `["docker.io/library/alpine:3"]`,
			}},
			Validations: []admissionregistrationv1.Validation{{Expression: allowedBaseImagesExpression}},
		},
	}
	compiled, errs := NewCompiler().Compile(policy, nil)
	require.Empty(t, errs)

	libCtx := &countingImageContext{FakeContextProvider: libs.NewFakeContextProvider()}
	metadata := map[string]any{"manifest": map[string]any{"annotations": map[string]any{
		"org.opencontainers.image.base.name": "docker.io/library/alpine:3",
	}}}
	libCtx.AddImageData("registry.test/one:v1", metadata)
	libCtx.AddImageData("registry.test/two:v1", metadata)

	evaluate := func(t *testing.T, images ...string) {
		t.Helper()
		containers := make([]any, 0, len(images))
		for _, image := range images {
			containers = append(containers, map[string]any{"name": "c", "image": image})
		}
		object := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "p", "namespace": "default"},
			"spec":       map[string]any{"containers": containers},
		}}
		attr := admission.NewAttributesRecord(object, nil, schema.GroupVersionKind{Version: "v1", Kind: "Pod"},
			"default", "p", schema.GroupVersionResource{Version: "v1", Resource: "pods"}, "", admission.Create, nil, false, nil)
		result, err := compiled.Evaluate(context.Background(), nil, attr, nil, nil, func() (map[string]any, error) { return map[string]any{}, nil }, libCtx)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Result, "message: %s", result.Message)
	}

	libCtx.calls.Store(0)
	evaluate(t, "registry.test/one:v1")
	assert.EqualValues(t, 1, libCtx.calls.Load(), "one container, four GetMetadata calls")

	libCtx.calls.Store(0)
	evaluate(t, "registry.test/one:v1", "registry.test/one:v1", "registry.test/two:v1")
	assert.EqualValues(t, 2, libCtx.calls.Load(), "two distinct images")

	libCtx.calls.Store(0)
	evaluate(t, "registry.test/one:v1")
	evaluate(t, "registry.test/one:v1")
	assert.EqualValues(t, 2, libCtx.calls.Load(), "separate evaluations fetch separately")
}
