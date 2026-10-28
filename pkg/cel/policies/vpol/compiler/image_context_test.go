package compiler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
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

type countingImageContext struct {
	*libs.FakeContextProvider
	calls atomic.Int64
}

func (provider *countingImageContext) GetImageData(image string, options []remote.Option) (map[string]any, error) {
	provider.calls.Add(1)
	return provider.FakeContextProvider.GetImageData(image, options)
}

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
		name        string
		ctx         context.Context
		evaluations [][]string
		wantCalls   int64
		wantErr     bool
	}{{
		name:        "one container, four GetMetadata calls fetch once",
		ctx:         context.Background(),
		evaluations: [][]string{{"198.51.100.10/one:v1"}},
		wantCalls:   1,
	}, {
		name:        "two distinct images fetch twice",
		ctx:         context.Background(),
		evaluations: [][]string{{"198.51.100.10/one:v1", "198.51.100.10/one:v1", "198.51.100.10/two:v1"}},
		wantCalls:   2,
	}, {
		name:        "separate evaluations fetch separately",
		ctx:         context.Background(),
		evaluations: [][]string{{"198.51.100.10/one:v1"}, {"198.51.100.10/one:v1"}},
		wantCalls:   2,
	}, {
		name:        "missing image is reported in the result",
		ctx:         context.Background(),
		evaluations: [][]string{{"198.51.100.10/missing:v1"}},
		wantCalls:   1,
		wantErr:     true,
	}, {
		name:        "cancelled context is reported in the result",
		ctx:         cancelled,
		evaluations: [][]string{{"198.51.100.10/one:v1"}},
		wantCalls:   0,
		wantErr:     true,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &countingImageContext{FakeContextProvider: libs.NewFakeContextProvider()}
			provider.AddImageData("198.51.100.10/one:v1", metadata)
			provider.AddImageData("198.51.100.10/two:v1", metadata)
			for _, images := range test.evaluations {
				result, err := evaluate(test.ctx, provider, images...)
				require.NoError(t, err)
				require.NotNil(t, result)
				if test.wantErr {
					assert.Error(t, result.Error)
					assert.False(t, result.Result)
				} else {
					assert.NoError(t, result.Error)
					assert.True(t, result.Result, "message: %s", result.Message)
				}
			}
			assert.EqualValues(t, test.wantCalls, provider.calls.Load())
		})
	}
}

type policyImageContext struct {
	libs.Context
	started chan context.Context
	abort   <-chan struct{}
}

func (p *policyImageContext) GetImageData(string, []remote.Option) (map[string]any, error) {
	return nil, errors.New("image callback is missing evaluation context")
}

func (p *policyImageContext) GetImageDataWithContext(ctx context.Context, image string, _ []remote.Option) (map[string]any, error) {
	if image == "blocked" {
		p.started <- ctx
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.abort:
			return nil, errors.New("test lookup stopped")
		}
	}
	return map[string]any{"resolvedImage": "ok"}, nil
}

func TestPolicyImageMetadataUsesEvaluationContext(t *testing.T) {
	libs.GetLibsCtx()
	t.Parallel()
	for _, location := range []string{"match condition", "variable"} {
		t.Run(location, func(t *testing.T) {
			t.Parallel()
			spec := policiesv1beta1.ValidatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
				Validations:             []admissionregistrationv1.Validation{{Expression: "true"}},
			}
			if location == "match condition" {
				spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "metadata", Expression: `image.getMetadata(object.image).resolvedImage == "ok"`}}
			} else {
				spec.Variables = []admissionregistrationv1.Variable{{Name: "metadata", Expression: `image.getMetadata(object.image).resolvedImage`}}
				spec.Validations[0].Expression = `variables.metadata == "ok"`
			}
			policy, errs := NewCompiler().Compile(&policiesv1beta1.ValidatingPolicy{Spec: spec}, nil)
			require.Empty(t, errs)
			abort := make(chan struct{})
			defer close(abort)
			provider := &policyImageContext{Context: libs.NewFakeContextProvider(), started: make(chan context.Context, 1), abort: abort}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				result, err := policy.Evaluate(ctx, map[string]any{"image": "blocked"}, nil, nil, nil, nil, provider)
				if err == nil && result != nil {
					err = result.Error
				}
				finished <- err
			}()
			select {
			case requestCtx := <-provider.started:
				assert.Same(t, ctx, requestCtx)
			case <-time.After(time.Second):
				t.Fatal("policy did not bind the image receiver before evaluation")
			}
			var calls sync.WaitGroup
			for range 4 {
				calls.Go(func() {
					result, err := policy.Evaluate(context.Background(), map[string]any{"image": "allowed"}, nil, nil, nil, nil, provider)
					assert.NoError(t, err)
					if assert.NotNil(t, result) {
						assert.NoError(t, result.Error)
						assert.True(t, result.Result)
					}
				})
			}
			calls.Wait()
			cancel()
			select {
			case err := <-finished:
				require.ErrorContains(t, err, "context canceled")
			case <-time.After(time.Second):
				t.Fatal("policy metadata lookup ignored cancellation")
			}
			result, err := policy.Evaluate(context.Background(), map[string]any{"image": "allowed"}, nil, nil, nil, nil, provider)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NoError(t, result.Error)
			assert.True(t, result.Result)
		})
	}
}
