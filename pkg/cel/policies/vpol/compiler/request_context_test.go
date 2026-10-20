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

// ctxAwareImageContext honors the evaluation context; libs detects these methods structurally.
type ctxAwareImageContext struct {
	*countingImageContext
}

func (c *ctxAwareImageContext) GetImageDataContext(ctx context.Context, image string, opts []remote.Option) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.GetImageData(image, opts)
}

func (c *ctxAwareImageContext) GetResourceContext(_ context.Context, apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return c.GetResource(apiVersion, resource, namespace, name)
}

func (c *ctxAwareImageContext) ListResourcesContext(_ context.Context, apiVersion, resource, namespace string, l map[string]string) (*unstructured.UnstructuredList, error) {
	return c.ListResources(apiVersion, resource, namespace, l)
}

// allowedBaseImages is the expression of the kyverno/policies allowed-base-images policy,
// which calls image.GetMetadata four times per container.
const allowedBaseImagesExpression = `object.spec.containers.all(container, has(image.GetMetadata(container.image).manifest.annotations) && image.GetMetadata(container.image).manifest.annotations != null && 'org.opencontainers.image.base.name' in image.GetMetadata(container.image).manifest.annotations && image.GetMetadata(container.image).manifest.annotations['org.opencontainers.image.base.name'] in variables.allowedBaseImages)`

func TestEvaluate_RequestScopedImageContext(t *testing.T) {
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

	metadata := map[string]any{"manifest": map[string]any{"annotations": map[string]any{
		"org.opencontainers.image.base.name": "docker.io/library/alpine:3",
	}}}
	evaluate := func(ctx context.Context, provider libs.Context, images ...string) (*EvaluationResult, error) {
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
		return compiled.Evaluate(ctx, nil, attr, nil, nil, func() (map[string]any, error) { return map[string]any{}, nil }, provider)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name         string
		ctx          context.Context
		contextAware bool
		evaluations  [][]string
		wantCalls    int64
		wantErr      bool
	}{{
		name:        "one container, four GetMetadata calls fetch once",
		ctx:         context.Background(),
		evaluations: [][]string{{"registry.test/one:v1"}},
		wantCalls:   1,
	}, {
		name:        "two distinct images fetch twice",
		ctx:         context.Background(),
		evaluations: [][]string{{"registry.test/one:v1", "registry.test/one:v1", "registry.test/two:v1"}},
		wantCalls:   2,
	}, {
		name:        "separate evaluations fetch separately",
		ctx:         context.Background(),
		evaluations: [][]string{{"registry.test/one:v1"}, {"registry.test/one:v1"}},
		wantCalls:   2,
	}, {
		name:        "missing image is reported in the result",
		ctx:         context.Background(),
		evaluations: [][]string{{"registry.test/missing:v1"}},
		wantCalls:   1,
		wantErr:     true,
	}, {
		name:         "cancelled context is reported in the result",
		ctx:          cancelled,
		contextAware: true,
		evaluations:  [][]string{{"registry.test/one:v1"}},
		wantCalls:    0,
		wantErr:      true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counting := &countingImageContext{FakeContextProvider: libs.NewFakeContextProvider()}
			counting.AddImageData("registry.test/one:v1", metadata)
			counting.AddImageData("registry.test/two:v1", metadata)
			var provider libs.Context = counting
			if tt.contextAware {
				provider = &ctxAwareImageContext{countingImageContext: counting}
			}
			for _, images := range tt.evaluations {
				result, err := evaluate(tt.ctx, provider, images...)
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.wantErr {
					assert.Error(t, result.Error)
					assert.False(t, result.Result)
				} else {
					assert.NoError(t, result.Error)
					assert.True(t, result.Result, "message: %s", result.Message)
				}
			}
			assert.EqualValues(t, tt.wantCalls, counting.calls.Load())
		})
	}
}
