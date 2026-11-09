package compiler

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

type targetCtxKey struct{}

// ctxRecordingContext implements the context aware methods that libs detects structurally.
type ctxRecordingContext struct {
	*libs.FakeContextProvider
	listCtx context.Context //nolint:containedctx // captured for assertions
}

func (c *ctxRecordingContext) GetImageDataContext(_ context.Context, image string, opts []remote.Option) (map[string]any, error) {
	return c.GetImageData(image, opts)
}

func (c *ctxRecordingContext) GetResourceContext(_ context.Context, apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return c.GetResource(apiVersion, resource, namespace, name)
}

func (c *ctxRecordingContext) ListResourcesContext(ctx context.Context, _, _, _ string, _ map[string]string) (*unstructured.UnstructuredList, error) {
	c.listCtx = ctx
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &unstructured.UnstructuredList{Object: map[string]any{"kind": "List"}}, nil
}

func TestEvaluateTargetExpression_UsesRequestContext(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "target-expr"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				Expression: `resource.List("v1", "configmaps", "default")`,
			},
		},
	}
	compiled, errs := NewCompiler().Compile(policy, nil)
	require.Empty(t, errs)

	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "p", "namespace": "default"},
	}}
	attr := admission.NewAttributesRecord(object, nil, schema.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "p", schema.GroupVersionResource{Version: "v1", Resource: "pods"}, "", admission.Create, nil, false, nil)

	tests := []struct {
		name    string
		cancel  bool
		wantErr error
	}{
		{name: "propagates request context"},
		{name: "cancelled request context fails evaluation", cancel: true, wantErr: context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &ctxRecordingContext{FakeContextProvider: libs.NewFakeContextProvider()}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), targetCtxKey{}, "request"))
			defer cancel()
			if tt.cancel {
				cancel()
			}
			out, err := compiled.EvaluateTargetExpression(ctx, attr, nil, nil, provider)
			require.NotNil(t, provider.listCtx, "resource.List must reach the bound request context provider")
			assert.Equal(t, "request", provider.listCtx.Value(targetCtxKey{}))
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr.Error())
				assert.Nil(t, out)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "List", out["kind"])
		})
	}
}
