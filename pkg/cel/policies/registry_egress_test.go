package policies_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	dpol "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	gpol "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	mpol "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	vpol "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

type imageLookupContext struct {
	*libs.FakeContextProvider
	calls int
}

func (provider *imageLookupContext) GetImageData(string, []remote.Option) (map[string]any, error) {
	provider.calls++
	return map[string]any{"resolvedImage": "fixture"}, nil
}

func TestPolicyMetadataGuardsWorkloadImages(t *testing.T) {
	previous := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = previous })
	compileProvider := libs.NewFakeContextProvider()
	libs.LibraryContext = compileProvider
	require.NoError(t, compileProvider.AddResource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
		&unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "fixture", "namespace": "default"}}}))
	for _, kind := range []string{"vpol", "mpol", "mpol-match", "mpol-target", "gpol", "dpol"} {
		for _, category := range []string{"containers", "initContainers", "ephemeralContainers"} {
			for _, authored := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/authored=%t", kind, category, authored), func(t *testing.T) {
					workloadImage := "169.254.169.254/workload:latest"
					object := &unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata":   map[string]any{"name": "pod", "namespace": "default"},
						"spec":       map[string]any{category: []any{map[string]any{"name": "app", "image": workloadImage}}},
					}}
					imageExpression := fmt.Sprintf("object.spec.%s[0].image", category)
					if authored {
						imageExpression = `"169.254.169.254/policy:latest"`
					}
					lookup := "image.GetMetadata(" + imageExpression + ").resolvedImage"
					condition := admissionregistrationv1.MatchCondition{Name: "image", Expression: lookup + ` == "fixture"`}
					attributes := admission.NewAttributesRecord(object, nil, schema.GroupVersionKind{Version: "v1", Kind: "Pod"},
						"default", "pod", schema.GroupVersionResource{Version: "v1", Resource: "pods"}, "", admission.Create, nil, false, nil)
					raw, err := json.Marshal(object.Object)
					require.NoError(t, err)
					request := &admissionv1.AdmissionRequest{Operation: admissionv1.Create, Object: runtime.RawExtension{Raw: raw}}
					provider := &imageLookupContext{FakeContextProvider: libs.NewFakeContextProvider()}
					ctx := context.Background()
					switch kind {
					case "vpol":
						policy, issues := vpol.NewCompiler().Compile(&policiesv1beta1.ValidatingPolicy{Spec: policiesv1beta1.ValidatingPolicySpec{
							Validations: []admissionregistrationv1.Validation{{Expression: condition.Expression}},
						}}, nil)
						require.Empty(t, issues)
						result, evaluationErr := policy.Evaluate(ctx, nil, attributes, request, nil, nil, provider)
						err = evaluationErr
						if err == nil {
							err = result.Error
						}
					case "mpol", "mpol-match", "mpol-target":
						policy, issues := mpol.NewCompiler().Compile(&policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
							MatchConditions: []admissionregistrationv1.MatchCondition{condition},
							TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
								Expression: `resource.get("v1", "configmaps", "default", ` + lookup + `)`,
							},
						}}, nil)
						require.Empty(t, issues)
						switch kind {
						case "mpol-match":
							require.Equal(t, authored, policy.MatchesConditions(ctx, attributes, request, nil, nil, provider))
						case "mpol-target":
							_, err = policy.EvaluateTargetExpression(ctx, attributes, request, nil, provider)
						default:
							err = policy.Evaluate(ctx, attributes, nil, *request, nil, nil, provider).Error
						}
					case "gpol":
						policy, issues := gpol.NewCompiler().Compile(&policiesv1beta1.GeneratingPolicy{Spec: policiesv1beta1.GeneratingPolicySpec{
							MatchConditions: []admissionregistrationv1.MatchCondition{condition},
						}}, nil)
						require.Empty(t, issues)
						_, err = policy.Evaluate(ctx, attributes, request, nil, provider)
					case "dpol":
						policy, issues := dpol.NewCompiler().Compile(&policiesv1beta1.DeletingPolicy{Spec: policiesv1beta1.DeletingPolicySpec{
							Schedule: "* * * * *", Conditions: []admissionregistrationv1.MatchCondition{condition},
						}}, nil)
						require.Empty(t, issues)
						_, err = policy.Evaluate(ctx, *object, nil, provider)
					}
					if authored {
						require.NoError(t, err)
						require.Positive(t, provider.calls)
					} else {
						if kind != "mpol-match" {
							require.ErrorContains(t, err, "connection blocked to network host")
						}
						require.Zero(t, provider.calls)
					}
				})
			}
		}
	}
}
