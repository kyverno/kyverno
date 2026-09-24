package evaluator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func TestCompiledPolicyConcurrentEvaluation(t *testing.T) {
	p := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
		EvaluationConfiguration:  &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false)},
		Validations:              []admissionregistrationv1.Validation{{Expression: "object.allowed == true"}},
	}}
	compiled, errs := NewCompiler(nil).Compile(p, nil)
	require.Empty(t, errs)
	for i := range 32 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			results := imageverify.NewImageVerificationResults()
			if i%2 == 0 {
				results.Record("image", true)
			}
			rt := &imageverify.Runtime{Results: results}
			for range 10 {
				result, err := compiled.Evaluate(context.Background(), rt, nil, map[string]any{"allowed": i%2 == 0}, nil, false, nil, nil)
				require.NoError(t, err)
				require.Equal(t, i%2 == 0, result.Result)
				if i%2 == 0 {
					require.NoError(t, compiled.EnforceRequired([]string{"image"}, results))
				} else {
					require.Error(t, compiled.EnforceRequired([]string{"image"}, results))
				}
			}
		})
	}
}

func TestCompiledPolicyRejectsNilRuntime(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
		Validations: []admissionregistrationv1.Validation{{Expression: "true"}},
	}}
	compiled, errs := NewCompiler(nil).Compile(policy, nil)
	require.Empty(t, errs)
	result, err := compiled.Evaluate(context.Background(), nil, nil, map[string]any{}, nil, false, nil, nil)
	require.Nil(t, result)
	require.ErrorContains(t, err, "image verification runtime is required")
	patches, err := compiled.MutateDigest(context.Background(), nil, nil, nil, nil, unstructured.Unstructured{}, nil, config.NewDefaultConfiguration(false), nil)
	require.Nil(t, patches)
	require.ErrorContains(t, err, "image verification runtime is required")
}

func TestRequiredEvidenceSharedAcrossPoliciesOnlyWithinRequest(t *testing.T) {
	first := &compiledPolicy{}
	second := &compiledPolicy{}
	for _, order := range [][2]*compiledPolicy{{first, second}, {second, first}} {
		results := imageverify.NewImageVerificationResults()
		require.Error(t, order[0].EnforceRequired([]string{"image"}, results))
		results.Record("image", true)
		for _, compiled := range order {
			require.NoError(t, compiled.EnforceRequired([]string{"image"}, results))
		}
		require.Error(t, order[0].EnforceRequired([]string{"image"}, imageverify.NewImageVerificationResults()))
	}
}

func TestCompiledPolicyUsesCurrentHTTPMocks(t *testing.T) {
	previous := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = previous })
	first := libs.NewFakeContextProvider()
	first.SetHTTPMocks(map[string]interface{}{"https://mock.invalid": map[string]any{"allowed": false}})
	second := libs.NewFakeContextProvider()
	second.SetHTTPMocks(map[string]interface{}{"https://mock.invalid": map[string]any{"allowed": true}})
	libs.LibraryContext = first
	p := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
		EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		Validations:             []admissionregistrationv1.Validation{{Expression: `http.Get("https://mock.invalid").allowed == true`}},
	}}
	compiled, errs := NewCompiler(nil).Compile(p, nil)
	require.Empty(t, errs)
	for i, test := range []struct {
		context libs.Context
		allowed bool
	}{{first, false}, {second, true}, {first, false}} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			result, err := compiled.Evaluate(context.Background(), &imageverify.Runtime{}, nil, map[string]any{}, nil, false, nil, test.context)
			require.NoError(t, err)
			require.Equal(t, test.allowed, result.Result)
		})
	}
}

type mutationImages struct{ imagedataloader.ImageContext }

func (mutationImages) Get(context.Context, string, []remote.Option, []name.Option) (*imagedataloader.ImageData, error) {
	image := &imagedataloader.ImageData{}
	image.Digest = "sha256:" + strings.Repeat("a", 64)
	return image, nil
}

func TestMutateDigestUsesCurrentHTTPMocks(t *testing.T) {
	first := libs.NewFakeContextProvider()
	first.SetHTTPMocks(map[string]interface{}{"https://mock.invalid": map[string]any{"allowed": false}})
	second := libs.NewFakeContextProvider()
	second.SetHTTPMocks(map[string]interface{}{"https://mock.invalid": map[string]any{"allowed": true}})
	policy := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
		MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "http", Expression: `http.Get("https://mock.invalid").allowed == true`}},
	}}
	compiled, errs := NewCompiler(nil).Compile(policy, nil)
	require.Empty(t, errs)
	request, attr, pod := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	require.NoError(t, unstructured.SetNestedSlice(pod.Object, []any{map[string]any{"name": "main", "image": "example.com/image:tag"}}, "spec", "containers"))
	for _, test := range []struct {
		context libs.Context
		patches int
	}{{first, 0}, {second, 1}, {first, 0}} {
		patches, err := compiled.MutateDigest(context.Background(), &imageverify.Runtime{ImageContext: mutationImages{}}, attr, request, nil, pod, nil, config.NewDefaultConfiguration(false), test.context)
		require.NoError(t, err)
		require.Len(t, patches, test.patches)
	}
}
