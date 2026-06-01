package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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
	invalidDpol     = &policiesv1beta1.DeletingPolicy{}
	namespaceMapper = func() meta.RESTMapper {
		m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
		m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, meta.RESTScopeRoot)
		return m
	}()
	dpol = &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-policy",
		},
		Spec: policiesv1beta1.DeletingPolicySpec{
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

	polex = &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-exception",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
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

	invalidCompiledPolicy, _ := comp.Compile(invalidDpol, []*policiesv1beta1.PolicyException{polex})
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
	policy := &policiesv1beta1.DeletingPolicy{
		Spec: policiesv1beta1.DeletingPolicySpec{
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

func TestHandleNamespaceWithNamespaceSelector(t *testing.T) {
	dpolNs := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cleanup-namespaces"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			MatchConstraints: &v1.MatchResources{
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{v1.OperationAll},
						Rule:       v1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"namespaces"}},
					},
				}},
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      "project-name",
						Operator: metav1.LabelSelectorOpExists,
					}},
				},
			},
			Conditions: []v1.MatchCondition{{Name: "always-true", Expression: "true"}},
		},
	}

	makeNs := func(name string, labels map[string]string) unstructured.Unstructured {
		u := unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("Namespace")
		u.SetName(name)
		u.SetLabels(labels)
		return u
	}

	nsResolverWith := func(name string, labels map[string]string) func(string) *corev1.Namespace {
		return func(ns string) *corev1.Namespace {
			if ns == name {
				return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
			}
			return nil
		}
	}

	compiledDpol, errs := comp.Compile(dpolNs, nil)
	assert.Empty(t, errs)
	pol := Policy{Policy: dpolNs, CompiledPolicy: compiledDpol}

	t.Run("namespace with matching label matches", func(t *testing.T) {
		labels := map[string]string{"project-name": "my-project"}
		engine := NewEngine(nsResolverWith("my-ns", labels), namespaceMapper, &libs.FakeContextProvider{}, matcher)
		resp, err := engine.Handle(ctx, pol, makeNs("my-ns", labels))
		assert.NoError(t, err)
		assert.True(t, resp.Match)
	})

	t.Run("namespace without matching label does not match", func(t *testing.T) {
		engine := NewEngine(nsResolverWith("kube-system", nil), namespaceMapper, &libs.FakeContextProvider{}, matcher)
		resp, err := engine.Handle(ctx, pol, makeNs("kube-system", nil))
		assert.NoError(t, err)
		assert.False(t, resp.Match)
	})
}

// TestHandleCleanupIntegrationTestNamespaces mirrors the real-world DeletingPolicy:
//
//	apiVersion: policies.kyverno.io/v1
//	kind: DeletingPolicy
//	metadata:
//	  name: cleanup-integration-test-namespaces
//	spec:
//	  schedule: "*/5 * * * *"
//	  matchConstraints:
//	    resourceRules:
//	      - apiGroups: [""]
//	        apiVersions: ["v1"]
//	        resources: ["namespaces"]
//	    namespaceSelector:
//	      matchExpressions:
//	        - key: project-name
//	          operator: Exists
func TestHandleCleanupIntegrationTestNamespaces(t *testing.T) {
	dpol := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cleanup-integration-test-namespaces"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule: "*/5 * * * *",
			MatchConstraints: &v1.MatchResources{
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{v1.OperationAll},
						Rule:       v1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"namespaces"}},
					},
				}},
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      "project-name",
						Operator: metav1.LabelSelectorOpExists,
					}},
				},
			},
			// No conditions — empty list matches all resources that pass the namespace selector.
		},
	}

	compiled, errs := comp.Compile(dpol, nil)
	assert.Empty(t, errs)
	pol := Policy{Policy: dpol, CompiledPolicy: compiled}

	makeNs := func(name string, labels map[string]string) unstructured.Unstructured {
		u := unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("Namespace")
		u.SetName(name)
		u.SetLabels(labels)
		return u
	}

	resolverFor := func(namespaces map[string]map[string]string) func(string) *corev1.Namespace {
		return func(name string) *corev1.Namespace {
			if labels, ok := namespaces[name]; ok {
				return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
			}
			return nil
		}
	}

	nsResolver := resolverFor(map[string]map[string]string{
		"acti-bifrost-even-env": {"project-name": "acti-bifrost-even-env"},
		"kube-system":           {"kubernetes.io/metadata.name": "kube-system"},
		"default":               {"kubernetes.io/metadata.name": "default"},
	})

	// noopResolver simulates CLI/cache-miss paths where the resolver always returns nil.
	noopResolver := func(string) *corev1.Namespace { return nil }

	engine := NewEngine(nsResolver, namespaceMapper, &libs.FakeContextProvider{}, matcher)

	tests := []struct {
		name      string
		nsName    string
		labels    map[string]string
		wantMatch bool
	}{
		{
			name:      "project namespace with project-name label is matched for cleanup",
			nsName:    "acti-bifrost-even-env",
			labels:    map[string]string{"project-name": "acti-bifrost-even-env"},
			wantMatch: true,
		},
		{
			name:      "kube-system without project-name label is not matched",
			nsName:    "kube-system",
			labels:    map[string]string{"kubernetes.io/metadata.name": "kube-system"},
			wantMatch: false,
		},
		{
			name:      "default namespace without project-name label is not matched",
			nsName:    "default",
			labels:    map[string]string{"kubernetes.io/metadata.name": "default"},
			wantMatch: false,
		},
		{
			name:      "project namespace with both labels is matched",
			nsName:    "my-project-ns",
			labels:    map[string]string{"project-name": "my-project-ns", "kubernetes.io/metadata.name": "my-project-ns"},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := engine.Handle(ctx, pol, makeNs(tt.nsName, tt.labels))
			assert.NoError(t, err)
			assert.Equal(t, tt.wantMatch, resp.Match)
		})
	}

	// Verify the fix holds even when the resolver returns nil (CLI / cache-miss paths):
	// ns must be built from the resource itself so namespaceSelector still works.
	noopEngine := NewEngine(noopResolver, namespaceMapper, &libs.FakeContextProvider{}, matcher)
	t.Run("project namespace matches with noop resolver (CLI path)", func(t *testing.T) {
		resp, err := noopEngine.Handle(ctx, pol, makeNs("acti-bifrost-even-env", map[string]string{"project-name": "acti-bifrost-even-env"}))
		assert.NoError(t, err)
		assert.True(t, resp.Match)
	})
	t.Run("system namespace not matched with noop resolver (CLI path)", func(t *testing.T) {
		resp, err := noopEngine.Handle(ctx, pol, makeNs("kube-system", map[string]string{"kubernetes.io/metadata.name": "kube-system"}))
		assert.NoError(t, err)
		assert.False(t, resp.Match)
	})
}

func TestHandleError(t *testing.T) {
	policy := &policiesv1beta1.DeletingPolicy{
		Spec: policiesv1beta1.DeletingPolicySpec{
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
