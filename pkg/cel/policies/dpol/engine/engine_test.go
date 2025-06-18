package engine

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
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

func TestHandle(t *testing.T) {

	policy := v1alpha1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-policy",
		},
		Spec: v1alpha1.DeletingPolicySpec{
			MatchConstraints: &v1.MatchResources{
				ResourceRules: []v1.NamedRuleWithOperations{
					{
						RuleWithOperations: v1.RuleWithOperations{
							Rule: v1.Rule{
								APIVersions: []string{"apps/v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
		}}

	polex := []*v1alpha1.PolicyException{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "e1"},
			Spec: v1alpha1.PolicyExceptionSpec{
				PolicyRefs: []v1alpha1.PolicyRef{
					{Name: "valid-policy", Kind: "DeletingPolicy"},
				},
			},
		},
	}

	compiler := compiler.NewCompiler()
	compiledPolicy, _ := compiler.Compile(&policy, polex)

	// Resource object
	resource := unstructured.Unstructured{}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	resource.SetName("my-pod")
	resource.SetNamespace("default")

	// GVK and GVR
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	groupVersion := schema.GroupVersion{Group: "apps", Version: "v1"}

	// Mapper
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{groupVersion})
	mapper.Add(gvk, meta.RESTScopeNamespace)
	mapper.AddSpecific(gvk, gvr, gvr, meta.RESTScopeNamespace)

	// Matcher
	matcher := matching.NewMatcher()

	// ns Resolver
	nsResolver := func(ns string) *corev1.Namespace {
		return nil
	}

	// Compiled Policy

	// Full Policy
	pol := Policy{
		CompiledPolicy: compiledPolicy,
		Policy:         policy,
	}

	// Engine
	engine := NewEngine(nsResolver, mapper, &fakeContext{}, matcher)

	resp, errs := engine.Handle(context.TODO(), pol, resource)

	assert.NoError(t, errs)
	assert.True(t, resp.Match)
	assert.Equal(t, &resource, resp.Resource)
}

/*

func TestHandle_NoMatch(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetAPIVersion("v1")
	resource.SetKind("Pod")
	resource.SetName("no-match-pod")
	resource.SetNamespace("default")
	gvk := resource.GroupVersionKind()

	mockMapper := new(MockMapper)
	mockMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}
	mockMapper.On("RESTMapping", gvk.GroupKind(), gvk.Version).Return(mockMapping, nil)

	mockMatcher := new(MockMatcher)
	mockMatcher.On("Match", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	mockEval := new(MockCompiledPolicy)
	// Should not be called

	policy := Policy{
		CompiledPolicy: mockEval,
		Policy: PolicyDefinition{
			Spec: PolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.ResourceRule{
						{
							Resources: []string{"pods"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(func(ns string) runtime.Object {
		return &unstructured.Unstructured{}
	}, mockMapper, libs.NewContext(), mockMatcher)

	resp, err := engine.Handle(context.Background(), policy, resource)

	assert.NoError(t, err)
	assert.False(t, resp.Match)
}

// Test for RESTMapping error
func TestHandle_RESTMappingError(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetAPIVersion("v1")
	resource.SetKind("Pod")
	resource.SetNamespace("default")
	gvk := resource.GroupVersionKind()

	mockMapper := new(MockMapper)
	mockMapper.On("RESTMapping", gvk.GroupKind(), gvk.Version).Return(nil, errors.New("mapping error"))

	engine := NewEngine(nil, mockMapper, libs.NewContext(), nil)

	policy := Policy{}

	resp, err := engine.Handle(context.Background(), policy, resource)

	assert.Error(t, err)
	assert.False(t, resp.Match)
	assert.Equal(t, EngineResponse{}, resp)
}
*/
