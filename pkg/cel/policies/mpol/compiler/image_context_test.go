package compiler

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type targetImageContext struct {
	libs.Context
	name     string
	requests []context.Context
}

func (p *targetImageContext) GetImageData(image string, options []remote.Option) (map[string]any, error) {
	return p.GetImageDataWithContext(context.Background(), image, options)
}

func (p *targetImageContext) GetImageDataWithContext(ctx context.Context, _ string, _ []remote.Option) (map[string]any, error) {
	p.requests = append(p.requests, ctx)
	return map[string]any{"provider": p.name}, nil
}

func (p *targetImageContext) GetResource(_, _, _, name string) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": name}}}, nil
}

func TestEvaluateTargetExpressionUsesRuntimeImageContext(t *testing.T) {
	previous := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = previous })
	for _, expression := range []string{"direct", "variable"} {
		t.Run(expression, func(t *testing.T) {
			compileProvider := &targetImageContext{Context: libs.NewFakeContextProvider(), name: "compile"}
			libs.LibraryContext = compileProvider
			spec := policiesv1beta1.MutatingPolicySpec{
				TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
					Expression: `resource.get("v1", "configmaps", "default", image.GetMetadata("registry.example/fixture:latest").provider)`,
				},
			}
			if expression == "variable" {
				spec.Variables = []admissionregistrationv1.Variable{{Name: "targetImage", Expression: `image.GetMetadata("registry.example/fixture:latest")`}}
				spec.TargetMatchConstraints.Expression = `resource.get("v1", "configmaps", "default", variables.targetImage.provider)`
			}
			policy, errs := NewCompiler().Compile(&policiesv1beta1.MutatingPolicy{Spec: spec}, nil)
			require.Empty(t, errs)
			// The processor's provider can differ both from the one captured
			// during compilation and from a subsequently replaced global.
			globalProvider := &targetImageContext{Context: libs.NewFakeContextProvider(), name: "global"}
			libs.LibraryContext = globalProvider
			runtimeProvider := &targetImageContext{Context: libs.NewFakeContextProvider(), name: "runtime"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result, err := policy.EvaluateTargetExpression(ctx, &mockAttributes{}, &admissionv1.AdmissionRequest{}, nil, runtimeProvider)
			require.NoError(t, err)
			require.Equal(t, map[string]any{"metadata": map[string]any{"name": "runtime"}}, result)
			require.Empty(t, compileProvider.requests)
			require.Empty(t, globalProvider.requests)
			require.Equal(t, []context.Context{ctx}, runtimeProvider.requests)
			cancel()
			_, err = policy.EvaluateTargetExpression(ctx, &mockAttributes{}, &admissionv1.AdmissionRequest{}, nil, runtimeProvider)
			require.ErrorContains(t, err, context.Canceled.Error())
			require.Len(t, runtimeProvider.requests, 1, "canceled evaluations must not start image requests")
		})
	}
}
