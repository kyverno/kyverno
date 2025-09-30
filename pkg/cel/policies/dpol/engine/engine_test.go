package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
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
		Policy:         dpol,
		CompiledPolicy: compiledDpol,
	}

	engine := NewEngine(nsResolver, mapper, &libs.FakeContextProvider{}, matcher)
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
		Policy:         invalidDpol,
		CompiledPolicy: invalidCompiledPolicy,
	}

	engine := NewEngine(nsResolver, invalidMapper, &libs.FakeContextProvider{}, matcher)
	resp, err := engine.Handle(ctx, pol, resource)

	assert.Error(t, err)
	assert.False(t, resp.Match)
}

func TestHandleConstraintsNil(t *testing.T) {
	policy := &policiesv1alpha1.DeletingPolicy{
		Spec: policiesv1alpha1.DeletingPolicySpec{
			MatchConstraints: nil,
		},
	}

	compiled, _ := comp.Compile(policy, nil)
	pol := Policy{
		Policy:         policy,
		CompiledPolicy: compiled,
	}

	resource := unstructured.Unstructured{}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	resource.SetName("test")
	resource.SetNamespace("default")

	mapper.Add(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, meta.RESTScopeNamespace)

	engine := NewEngine(nsResolver, mapper, &libs.FakeContextProvider{}, matcher)
	resp, err := engine.Handle(ctx, pol, resource)

	assert.NoError(t, err)
	assert.False(t, resp.Match)
}

func TestHandleError(t *testing.T) {
	policy := &policiesv1alpha1.DeletingPolicy{
		Spec: policiesv1alpha1.DeletingPolicySpec{
			MatchConstraints: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "key ", Operator: "In", Values: []string{"bad value"}}}},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"deployments"}},
						Operations: []v1.OperationType{"*"},
					},
				}},
			},
		},
	}

	compiled, _ := comp.Compile(policy, nil)
	pol := Policy{
		Policy:         policy,
		CompiledPolicy: compiled,
	}

	resource := unstructured.Unstructured{}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	resource.SetName("test")
	resource.SetNamespace("default")

	mapper.Add(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, meta.RESTScopeNamespace)

	engine := NewEngine(nsResolver, mapper, &libs.FakeContextProvider{}, matcher)
	resp, err := engine.Handle(ctx, pol, resource)

	assert.Error(t, err)
	assert.False(t, resp.Match)
}
