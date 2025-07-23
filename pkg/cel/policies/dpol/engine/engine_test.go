package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// mock Context
type fakeContext struct{}

func (f *fakeContext) GenerateResources(string, []map[string]any) error        { return nil }
func (f *fakeContext) GetGlobalReference(name, projection string) (any, error) { return name, nil }
func (f *fakeContext) GetImageData(image string) (map[string]any, error) {
	return map[string]any{"test": image}, nil
}
func (f *fakeContext) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{}, nil
}
func (f *fakeContext) GetGeneratedResources() []*unstructured.Unstructured { return nil }
func (f *fakeContext) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ClearGeneratedResources() {}
func (f *fakeContext) SetGenerateContext(polName, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache bool) {
	panic("not implemented")
}

var (
	invalidDpol = &policiesv1alpha1.DeletingPolicy{}
	dpol        = &policiesv1alpha1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-policy",
		},
		Spec: policiesv1alpha1.DeletingPolicySpec{
			MatchConstraints: &v1.MatchResources{
				ResourceRules: []v1.NamedRuleWithOperations{
					{
						RuleWithOperations: v1.RuleWithOperations{
							Operations: []v1.OperationType{
								v1.OperationAll,
							},
							Rule: v1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
			Conditions: []v1.MatchCondition{
				{
					Name:       "always-true",
					Expression: "true",
				},
			},
		},
	}

	mapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{
			Group:   "apps",
			Version: "v1",
		},
	})

	invalidMapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{
			Group:   "invalidGroup",
			Version: "invalidVersion",
		},
	})

	polex = &policiesv1alpha1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-exception",
		},
		Spec: policiesv1alpha1.PolicyExceptionSpec{
			// add req. fields if required
		},
	}

	ctx      = context.Background()
	resource = unstructured.Unstructured{}
	comp     = compiler.NewCompiler()

	matcher    = matching.NewMatcher()
	nsResolver = func(ns string) *corev1.Namespace { return nil }
)

func TestHandleValidPolicy(t *testing.T) {
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	resource.SetName("nginx")
	resource.SetNamespace("default")

	mapper.Add(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, meta.RESTScopeNamespace)

	compiledDpol, _ := comp.Compile(dpol, nil)

	pol := Policy{
		*dpol,
		compiledDpol,
	}

	engine := NewEngine(nsResolver, mapper, &fakeContext{}, matcher)
	resp, err := engine.Handle(ctx, pol, resource)

	assert.NoError(t, err)
	assert.True(t, resp.Match)
}

func TestHandleWithPolex(t *testing.T) {
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	resource.SetName("nginx")
	resource.SetNamespace("default")

	mapper.Add(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v2",
		Kind:    "Deployment",
	}, meta.RESTScopeNamespace)

	invalidCompiledPolicy, _ := comp.Compile(invalidDpol, []*policiesv1alpha1.PolicyException{polex})
	pol := Policy{
		*invalidDpol,
		invalidCompiledPolicy,
	}

	engine := NewEngine(nsResolver, invalidMapper, &fakeContext{}, matcher)
	resp, err := engine.Handle(ctx, pol, resource)

	assert.Error(t, err)
	assert.False(t, resp.Match)
}
