package matching

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/apis/example"
	"k8s.io/apiserver/pkg/authentication/user"
)

func TestNewMatcher(t *testing.T) {
	matcher := NewMatcher()
	assert.NotNil(t, matcher)
}

var _ matching.MatchCriteria = &fakeCriteria{}

type fakeCriteria struct {
	matchResources v1.MatchResources
}

func (fc *fakeCriteria) GetMatchResources() v1.MatchResources {
	return fc.matchResources
}

func (fc *fakeCriteria) GetParsedNamespaceSelector() (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(fc.matchResources.NamespaceSelector)
}

func (fc *fakeCriteria) GetParsedObjectSelector() (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(fc.matchResources.ObjectSelector)
}

func gvr(group, version, resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

func gvk(group, version, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
}

func TestOfflineMatcher(t *testing.T) {
	a := NewOfflineMatcher()
	equivalent := v1.Equivalent
	exact := v1.Exact

	// TODO write test cases for name matching and exclude matching
	testcases := []struct {
		name string

		criteria *v1.MatchResources
		attrs    admission.Attributes

		expectMatches bool
		expectErr     string
	}{
		{
			name:          "no rules (just write)",
			criteria:      &v1.MatchResources{NamespaceSelector: &metav1.LabelSelector{}, ResourceRules: []v1.NamedRuleWithOperations{}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "wildcard rule, match as requested",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"*"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "specific rules, prefer exact match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "specific rules, equivalent version match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "specific rules, exact match miss",
			criteria: &v1.MatchResources{
				MatchPolicy:       &exact,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "equivalent version matches custom resource across versions",
			criteria: &v1.MatchResources{
				MatchPolicy:       &equivalent,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{v1.Create, v1.Update},
						Rule: v1.Rule{
							APIGroups:   []string{"policies.kyverno.io"},
							APIVersions: []string{"v1"},
							Resources:   []string{"namespacedvalidatingpolicies"},
						},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("policies.kyverno.io", "v1beta1", "NamespacedValidatingPolicy"), "ns", "name", gvr("policies.kyverno.io", "v1beta1", "namespacedvalidatingpolicies"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "equivalent cross-group resource match",
			criteria: &v1.MatchResources{
				MatchPolicy:       &equivalent,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{v1.Create},
						Rule: v1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("extensions", "v1beta1", "Deployment"), "ns", "name", gvr("extensions", "v1beta1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "exact cross-group resource mismatch",
			criteria: &v1.MatchResources{
				MatchPolicy:       &exact,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{v1.Create},
						Rule: v1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("extensions", "v1beta1", "Deployment"), "ns", "name", gvr("extensions", "v1beta1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "specific rules, subresource prefer exact match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "scale", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "specific rules, equivalent subresource version match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "scale", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "specific rules, subresource exact match miss",
			criteria: &v1.MatchResources{
				MatchPolicy:       &exact,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}, {
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "scale", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "specific rules, prefer exact match and name match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"name"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "specific rules, prefer exact match and name match miss",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"wrong-name"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "specific rules, subresource equivalent match, prefer extensions and name match miss",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"wrong-name"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}}},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("extensions", "v1beta1", "deployments"), "scale", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "exclude resource exact group mismatch",
			criteria: &v1.MatchResources{
				MatchPolicy:       &exact,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"*"}},
					},
				}},
				ExcludeResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "exclude resource miss on match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"*"}},
					},
				}},
				ExcludeResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("extensions", "v1beta1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "treat empty ResourceRules as match with exact exclusion",
			criteria: &v1.MatchResources{
				MatchPolicy:       &exact,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ExcludeResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"extensions"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "treat non-empty ResourceRules as no match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules:     []v1.NamedRuleWithOperations{{}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "erroring namespace selector on otherwise non-matching rule doesn't error",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "key ", Operator: "In", Values: []string{"bad value"}}}},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"deployments"}},
						Operations: []v1.OperationType{"*"},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(&example.Pod{}, nil, gvk("example.apiserver.k8s.io", "v1", "Pod"), "ns", "name", gvr("example.apiserver.k8s.io", "v1", "pods"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
			expectErr:     "",
		},
		{
			name: "erroring namespace selector on otherwise matching rule errors",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "key", Operator: "In", Values: []string{"bad value"}}}},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"pods"}},
						Operations: []v1.OperationType{"*"},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(&example.Pod{}, nil, gvk("example.apiserver.k8s.io", "v1", "Pod"), "ns", "name", gvr("example.apiserver.k8s.io", "v1", "pods"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
			expectErr:     "bad value",
		},
		{
			name: "erroring object selector on otherwise non-matching rule doesn't error",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "key", Operator: "In", Values: []string{"bad value"}}}},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"deployments"}},
						Operations: []v1.OperationType{"*"},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(&example.Pod{}, nil, gvk("example.apiserver.k8s.io", "v1", "Pod"), "ns", "name", gvr("example.apiserver.k8s.io", "v1", "pods"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
			expectErr:     "",
		},
		{
			name: "erroring object selector on otherwise matching rule errors",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "key", Operator: "In", Values: []string{"bad value"}}}},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Rule:       v1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"pods"}},
						Operations: []v1.OperationType{"*"},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(&example.Pod{}, nil, gvk("example.apiserver.k8s.io", "v1", "Pod"), "ns", "name", gvr("example.apiserver.k8s.io", "v1", "pods"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
			expectErr:     "bad value",
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			matches, err := a.Match(&fakeCriteria{matchResources: *testcase.criteria}, testcase.attrs, nil)
			if err != nil {
				if len(testcase.expectErr) == 0 {
					t.Fatal(err)
				}
				if !strings.Contains(err.Error(), testcase.expectErr) {
					t.Fatalf("expected error containing %q, got %s", testcase.expectErr, err.Error())
				}
				return
			} else if len(testcase.expectErr) > 0 {
				t.Fatalf("expected error %q, got no error", testcase.expectErr)
			}

			if matches != testcase.expectMatches {
				t.Fatalf("expected matches = %v; got %v", testcase.expectMatches, matches)
			}
		})
	}
}

func TestMatcherEquivalentResources(t *testing.T) {
	t.Parallel()
	equivalent, exact := v1.Equivalent, v1.Exact
	tests := []struct {
		name           string
		request        schema.GroupVersionResource
		rule           schema.GroupVersionResource
		subresource    string
		wantEquivalent bool
		wantExact      bool
	}{
		{"same version", gvr("apps", "v1", "deployments"), gvr("apps", "v1", "deployments"), "", true, true},
		{"same group across versions", gvr("apps", "v1", "deployments"), gvr("apps", "v1beta1", "deployments"), "", true, false},
		{"older request against newer rule", gvr("apps", "v1beta1", "deployments"), gvr("apps", "v1", "deployments"), "", true, false},
		{"unregistered same-group rule version", gvr("apps", "v1", "deployments"), gvr("apps", "v99", "deployments"), "", true, false},
		{"unregistered migrated rule version", gvr("apps", "v1", "deployments"), gvr("extensions", "v99", "deployments"), "", true, false},
		{"custom resource across versions", gvr("example.com", "v1", "widgets"), gvr("example.com", "v1beta1", "widgets"), "", true, false},
		{"older custom resource request against newer rule", gvr("example.com", "v1beta1", "widgets"), gvr("example.com", "v1", "widgets"), "", true, false},
		{"known cross-group equivalent", gvr("extensions", "v1beta1", "deployments"), gvr("apps", "v1", "deployments"), "", true, false},
		{"known cross-group equivalent reverse", gvr("apps", "v1", "deployments"), gvr("extensions", "v1beta1", "deployments"), "", true, false},
		{"known networking equivalent", gvr("networking.k8s.io", "v1", "ingresses"), gvr("extensions", "v1beta1", "ingresses"), "", true, false},
		{"known events equivalent", gvr("events.k8s.io", "v1", "events"), gvr("", "v1", "events"), "", true, false},
		{"replicationcontroller equivalent", gvr("extensions", "v1beta1", "replicationcontrollers"), gvr("", "v1", "replicationcontrollers"), "", true, false},
		{"replicationcontroller equivalent reverse", gvr("", "v1", "replicationcontrollers"), gvr("extensions", "v1beta1", "replicationcontrollers"), "", true, false},
		{"replicationcontroller scale equivalent", gvr("extensions", "v1beta1", "replicationcontrollers"), gvr("", "v1", "replicationcontrollers/scale"), "scale", true, false},
		{"replicationcontroller scale equivalent reverse", gvr("", "v1", "replicationcontrollers"), gvr("extensions", "v1beta1", "replicationcontrollers/scale"), "scale", true, false},
		{"replicationcontroller subresource mismatch", gvr("", "v1", "replicationcontrollers"), gvr("extensions", "v1beta1", "replicationcontrollers/status"), "scale", false, false},
		{"podsecuritypolicy equivalent", gvr("extensions", "v1beta1", "podsecuritypolicies"), gvr("policy", "v1beta1", "podsecuritypolicies"), "", true, false},
		{"podsecuritypolicy equivalent reverse", gvr("policy", "v1beta1", "podsecuritypolicies"), gvr("extensions", "v1beta1", "podsecuritypolicies"), "", true, false},
		{"unrelated replicationcontroller group", gvr("example.com", "v1", "replicationcontrollers"), gvr("", "v1", "replicationcontrollers"), "", false, false},
		{"unrelated podsecuritypolicy group", gvr("example.com", "v1beta1", "podsecuritypolicies"), gvr("policy", "v1beta1", "podsecuritypolicies"), "", false, false},
		{"unrelated request group", gvr("example.com", "v1", "deployments"), gvr("apps", "v1", "deployments"), "", false, false},
		{"unrelated rule group", gvr("apps", "v1", "deployments"), gvr("example.com", "v1", "deployments"), "", false, false},
		{"unrelated custom groups", gvr("one.example.com", "v1", "widgets"), gvr("two.example.com", "v1beta1", "widgets"), "", false, false},
		{"unrelated resource in migrated groups", gvr("apps", "v1", "widgets"), gvr("extensions", "v1beta1", "widgets"), "", false, false},
		{"different resource", gvr("apps", "v1", "deployments"), gvr("apps", "v1beta1", "replicasets"), "", false, false},
		{"known equivalent subresource", gvr("apps", "v1", "deployments"), gvr("extensions", "v1beta1", "deployments/scale"), "scale", true, false},
		{"unrelated subresource group", gvr("example.com", "v1", "deployments"), gvr("apps", "v1", "deployments/scale"), "scale", false, false},
		{"subresource mismatch", gvr("apps", "v1", "deployments"), gvr("extensions", "v1beta1", "deployments/status"), "scale", false, false},
		{"wildcard group across versions", gvr("example.com", "v1beta1", "deployments"), gvr("*", "v1", "deployments"), "", true, false},
		{"wildcard resource unrelated group", gvr("example.com", "v1", "deployments"), gvr("apps", "v1", "*"), "", false, false},
		{"wildcard version unrelated group", gvr("example.com", "v1", "deployments"), gvr("apps", "*", "deployments"), "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, policy := range []*v1.MatchPolicyType{nil, &equivalent, &exact} {
				for _, exclude := range []bool{false, true} {
					rule := v1.NamedRuleWithOperations{
						RuleWithOperations: v1.RuleWithOperations{
							Operations: []v1.OperationType{v1.Create},
							Rule: v1.Rule{
								APIGroups:   []string{tt.rule.Group},
								APIVersions: []string{tt.rule.Version},
								Resources:   []string{tt.rule.Resource},
							},
						},
					}
					criteria := &fakeCriteria{matchResources: v1.MatchResources{
						MatchPolicy:       policy,
						NamespaceSelector: &metav1.LabelSelector{},
						ObjectSelector:    &metav1.LabelSelector{},
					}}
					if exclude {
						criteria.matchResources.ExcludeResourceRules = []v1.NamedRuleWithOperations{rule}
					} else {
						criteria.matchResources.ResourceRules = []v1.NamedRuleWithOperations{rule}
					}
					original := criteria.matchResources.DeepCopy()
					attr := admission.NewAttributesRecord(nil, nil, gvk(tt.request.Group, tt.request.Version, "Deployment"), "ns", "name", tt.request, tt.subresource, admission.Create, nil, false, nil)
					for _, mode := range []struct {
						name    string
						matcher Matcher
						offline bool
					}{
						{"runtime", NewMatcher(), false},
						{"nil mapper", NewMatcherWithEquivalentResourceMapper(nil), false},
						{"offline", NewOfflineMatcher(), true},
					} {
						want := tt.wantExact
						if mode.offline && policy != &exact {
							want = tt.wantEquivalent
						}
						reason := "is not covered by the policy's resourceRules"
						if exclude {
							want = !want
							reason = "is excluded by the policy's excludeResourceRules"
						}
						if want {
							reason = "matched kind Deployment"
						}
						matched, err := mode.matcher.Match(criteria, attr, nil)
						assert.NoError(t, err)
						assert.Equal(t, want, matched, "mode=%s policy=%v exclude=%v", mode.name, policy, exclude)
						assert.Contains(t, ExplainWithMatcher(mode.matcher, &criteria.matchResources, attr, nil, matched), reason)
						assert.Equal(t, original, &criteria.matchResources, "matching must not mutate rules")
					}
				}
			}
		})
	}
}

func TestMatcherWithEquivalentResourceMapper(t *testing.T) {
	t.Parallel()
	legacy := gvr("legacy.example.com", "v1beta1", "oldwidgets")
	current := gvr("current.example.com", "v1", "widgets")
	registry := runtime.NewEquivalentResourceRegistryWithIdentity(func(schema.GroupResource) string { return "widgets" })
	for _, resource := range []schema.GroupVersionResource{legacy, current} {
		registry.RegisterKindFor(resource, "", resource.GroupVersion().WithKind("Widget"))
		registry.RegisterKindFor(resource, "scale", gvk("autoscaling", "v1", "Scale"))
	}
	m := NewMatcherWithEquivalentResourceMapper(registry)
	equivalent, exact := v1.Equivalent, v1.Exact
	clusterScope := v1.ClusterScope
	tests := []struct {
		name        string
		request     schema.GroupVersionResource
		rule        schema.GroupVersionResource
		subresource string
		operation   v1.OperationType
		scope       *v1.ScopeType
		names       []string
		want        bool
		wantExact   bool
	}{
		{name: "registered cross-group and cross-plural equivalent", request: legacy, rule: current, want: true},
		{name: "registered equivalent reverse", request: current, rule: legacy, want: true},
		{name: "exact match without registration", request: gvr("other.example.com", "v1", "widgets"), rule: gvr("other.example.com", "v1", "widgets"), want: true, wantExact: true},
		{name: "request versions use the mapper's group-resource identity", request: gvr("current.example.com", "v2", "widgets"), rule: current, want: true},
		{name: "unregistered rule version", request: current, rule: gvr("current.example.com", "v2", "widgets")},
		{name: "unregistered group with same plural", request: gvr("other.example.com", "v1", "widgets"), rule: current},
		{name: "built-in migration has no local fallback", request: gvr("extensions", "v1beta1", "deployments"), rule: gvr("apps", "v1", "deployments")},
		{name: "registered subresource", request: legacy, rule: gvr(current.Group, current.Version, "widgets/scale"), subresource: "scale", want: true},
		{name: "registered wildcard subresource", request: legacy, rule: gvr(current.Group, current.Version, "widgets/*"), subresource: "scale", want: true},
		{name: "unregistered subresource", request: legacy, rule: gvr(current.Group, current.Version, "widgets/status"), subresource: "status"},
		{name: "operation mismatch", request: legacy, rule: current, operation: v1.Update},
		{name: "scope mismatch", request: legacy, rule: current, scope: &clusterScope},
		{name: "name mismatch", request: legacy, rule: current, names: []string{"other"}},
		{name: "name match", request: legacy, rule: current, names: []string{"name"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, policy := range []*v1.MatchPolicyType{nil, &equivalent, &exact} {
				for _, exclude := range []bool{false, true} {
					operation := tt.operation
					if operation == "" {
						operation = v1.Create
					}
					rule := v1.NamedRuleWithOperations{
						ResourceNames: tt.names,
						RuleWithOperations: v1.RuleWithOperations{
							Operations: []v1.OperationType{operation},
							Rule: v1.Rule{
								APIGroups:   []string{tt.rule.Group},
								APIVersions: []string{tt.rule.Version},
								Resources:   []string{tt.rule.Resource},
								Scope:       tt.scope,
							},
						},
					}
					constraints := &v1.MatchResources{MatchPolicy: policy}
					want := tt.want
					if policy == &exact {
						want = tt.wantExact
					}
					reason := "is not covered by the policy's resourceRules"
					if exclude {
						constraints.ExcludeResourceRules = []v1.NamedRuleWithOperations{rule}
						want = !want
						reason = "is excluded by the policy's excludeResourceRules"
					} else {
						constraints.ResourceRules = []v1.NamedRuleWithOperations{rule}
					}
					if want {
						reason = "matched kind Widget"
					}
					original := constraints.DeepCopy()
					attr := admission.NewAttributesRecord(nil, nil, tt.request.GroupVersion().WithKind("Widget"), "ns", "name", tt.request, tt.subresource, admission.Create, nil, false, nil)
					matched, err := m.Match(&MatchCriteria{Constraints: constraints}, attr, nil)
					assert.NoError(t, err)
					assert.Equal(t, want, matched, "policy=%v exclude=%v", policy, exclude)
					assert.Contains(t, ExplainWithEquivalentResourceMapper(constraints, attr, nil, matched, registry), reason)
					assert.Equal(t, original, constraints, "matching must not mutate rules")
				}
			}
		})
	}
}

func TestOfflineMatcherRequiresConversion(t *testing.T) {
	t.Parallel()
	equivalent, exact := v1.Equivalent, v1.Exact
	rule := v1.NamedRuleWithOperations{RuleWithOperations: v1.RuleWithOperations{
		Operations: []v1.OperationType{v1.OperationAll},
		Rule: v1.Rule{
			APIGroups: []string{"apps"}, APIVersions: []string{"v1beta1"}, Resources: []string{"deployments"},
		},
	}}
	exactRule := *rule.DeepCopy()
	exactRule.APIVersions = []string{"v1"}
	wrongName := *rule.DeepCopy()
	wrongName.ResourceNames = []string{"other"}
	tests := []struct {
		name    string
		policy  *v1.MatchPolicyType
		rules   []v1.NamedRuleWithOperations
		exclude []v1.NamedRuleWithOperations
		oldOnly bool
		want    bool
		wantErr bool
	}{
		{name: "create conversion unavailable", policy: &equivalent, rules: []v1.NamedRuleWithOperations{rule}, wantErr: true},
		{name: "default conversion unavailable", rules: []v1.NamedRuleWithOperations{rule}, wantErr: true},
		{name: "old object conversion unavailable", rules: []v1.NamedRuleWithOperations{rule}, oldOnly: true, wantErr: true},
		{name: "exact mismatch stays skipped", policy: &exact, rules: []v1.NamedRuleWithOperations{rule}},
		{name: "name mismatch stays skipped", rules: []v1.NamedRuleWithOperations{wrongName}},
		{name: "later exact rule needs no conversion", rules: []v1.NamedRuleWithOperations{rule, exactRule}, want: true},
		{name: "equivalent exclusion needs no conversion", rules: []v1.NamedRuleWithOperations{rule}, exclude: []v1.NamedRuleWithOperations{rule}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			obj := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]interface{}{"name": "deployment", "namespace": "default"},
				"spec":     map[string]interface{}{"newField": "not present in the old schema"},
			}}
			original := obj.DeepCopy()
			var object, oldObject runtime.Object = obj, nil
			op := admission.Create
			if tt.oldOnly {
				object, oldObject, op = nil, obj, admission.Delete
			}
			attr := admission.NewAttributesRecord(object, oldObject, gvk("apps", "v1", "Deployment"), "default", "deployment", gvr("apps", "v1", "deployments"), "", op, nil, false, nil)
			constraints := &v1.MatchResources{MatchPolicy: tt.policy, ResourceRules: tt.rules, ExcludeResourceRules: tt.exclude}
			matched, err := NewOfflineMatcher().Match(&MatchCriteria{Constraints: constraints}, attr, nil)
			if tt.wantErr {
				assert.ErrorContains(t, err, "API-version conversion is unavailable")
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, matched)
			assert.Equal(t, original, obj, "matching must not relabel or mutate objects")
		})
	}
}

func TestEquivalentScaleConversion(t *testing.T) {
	t.Parallel()
	resource := gvr("apps", "v1", "deployments")
	equivalent := gvr("extensions", "v1beta1", "deployments")
	kind := gvk("autoscaling", "v1", "Scale")
	for _, targetKind := range []schema.GroupVersionKind{kind, gvk("autoscaling", "v2", "Scale")} {
		t.Run(targetKind.String(), func(t *testing.T) {
			t.Parallel()
			registry := runtime.NewEquivalentResourceRegistryWithIdentity(func(resource schema.GroupResource) string {
				return equivalentGroupResource(resource).String()
			})
			registry.RegisterKindFor(resource, "scale", kind)
			registry.RegisterKindFor(equivalent, "scale", targetKind)
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(kind)
			attr := admission.NewAttributesRecord(obj, nil, kind, "default", "deployment", resource, "scale", admission.Update, nil, false, nil)
			constraints := &v1.MatchResources{ResourceRules: []v1.NamedRuleWithOperations{{RuleWithOperations: v1.RuleWithOperations{
				Operations: []v1.OperationType{v1.Update},
				Rule: v1.Rule{
					APIGroups: []string{equivalent.Group}, APIVersions: []string{equivalent.Version}, Resources: []string{"deployments/scale"},
				},
			}}}}
			matched, err := NewMatcherWithEquivalentResourceMapper(registry).Match(&MatchCriteria{Constraints: constraints}, attr, nil)
			if targetKind == kind {
				assert.NoError(t, err)
				assert.True(t, matched, "equivalent scale endpoints already use the same object representation")
			} else {
				assert.ErrorContains(t, err, "API-version conversion is unavailable")
				assert.False(t, matched)
			}
		})
	}
}

func Test_matchesResourceRules(t *testing.T) {
	tests := []struct {
		name         string
		object       runtime.Object
		resourceName string
		expected     bool
	}{
		{
			name: "match using generateName prefix",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "test-prefix-",
					},
				},
			},
			resourceName: "test-prefix",
			expected:     true,
		},
		{
			name:         "no match when object is nil and name is empty",
			object:       nil,
			resourceName: "test-prefix",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := admission.NewAttributesRecord(
				tt.object,
				nil,
				schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				"default", // namespace
				"",        // name is empty to trigger generateName fallback
				schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				"", // subresource
				admission.Create,
				nil,   // operationOptions
				false, // dryRun
				&user.DefaultInfo{Name: "test-user"},
			)

			namedRules := []v1.NamedRuleWithOperations{
				{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"CREATE"},
						Rule: v1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Resources:   []string{"*/*"},
						},
					},
					ResourceNames: []string{tt.resourceName},
				},
			}

			match, err := matchesResourceRules(namedRules, attr, nil, nil, true)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, match)
		})
	}
}
