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

func TestMatcher(t *testing.T) {
	a := NewMatcher()

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
			name: "specific rules, match miss",
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
			expectMatches: false,
		},
		{
			name: "specific rules, exact match miss",
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
			name: "specific rules, subresource match miss",
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
			expectMatches: false,
		},
		{
			name: "specific rules, subresource exact match miss",
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
			name: "exclude resource match on miss",
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
			name: "generateName matches in resourceRules when name is empty",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deploy"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs: admission.NewAttributesRecord(
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"generateName": "deploy-",
						},
					},
				},
				nil, gvk("apps", "v1", "Deployment"), "ns", "", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "generateName in excludeResourceRules does not exclude when name is empty",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
				ExcludeResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deploy"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs: admission.NewAttributesRecord(
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"generateName": "deploy-",
						},
					},
				},
				nil, gvk("apps", "v1", "Deployment"), "ns", "", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "generateName prefix matching preserved in resourceRules",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deploy"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs: admission.NewAttributesRecord(
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"generateName": "deploy-attacker-",
						},
					},
				},
				nil, gvk("apps", "v1", "Deployment"), "ns", "", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "exact name in excludeResourceRules excludes",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
				ExcludeResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deploy"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs: admission.NewAttributesRecord(
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"name": "deploy",
						},
					},
				},
				nil, gvk("apps", "v1", "Deployment"), "ns", "deploy", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "treat empty ResourceRules as match",
			criteria: &v1.MatchResources{
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
		{
			name: "name matching with single name - match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"my-deployment"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "my-deployment", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "name matching with single name - no match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"my-deployment"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "other-deployment", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "name matching with multiple names - match first",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deployment-1", "deployment-2", "deployment-3"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "deployment-1", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "name matching with multiple names - match middle",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deployment-1", "deployment-2", "deployment-3"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "deployment-2", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "name matching with multiple names - match last",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deployment-1", "deployment-2", "deployment-3"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "deployment-3", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "name matching with multiple names - no match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"deployment-1", "deployment-2", "deployment-3"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "deployment-4", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "name matching with empty resource names - matches all",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "any-deployment", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "exclude matching with resource name - exclude matches",
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
					ResourceNames: []string{"excluded-deployment"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "excluded-deployment", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "exclude matching with resource name - exclude does not match",
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
					ResourceNames: []string{"excluded-deployment"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "allowed-deployment", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "exclude matching with multiple resource names - exclude matches one",
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
					ResourceNames: []string{"excluded-1", "excluded-2", "excluded-3"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "excluded-2", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "exclude matching with multiple resource names - exclude does not match",
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
					ResourceNames: []string{"excluded-1", "excluded-2", "excluded-3"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "allowed-deployment", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "exclude matching with empty resource names - excludes all matching resources",
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
					ResourceNames: []string{},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("apps", "v1", "Deployment"), "ns", "any-deployment", gvr("apps", "v1", "deployments"), "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "name matching with subresource - match",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"my-deployment"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "my-deployment", gvr("apps", "v1", "deployments"), "scale", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name: "name matching with subresource - no match on name",
			criteria: &v1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []v1.NamedRuleWithOperations{{
					ResourceNames: []string{"my-deployment"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "other-deployment", gvr("apps", "v1", "deployments"), "scale", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name: "exclude matching with subresource - exclude matches",
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
					ResourceNames: []string{"excluded-deployment"},
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{"*"},
						Rule:       v1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments", "deployments/scale"}},
					},
				}},
			},
			attrs:         admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "excluded-deployment", gvr("apps", "v1", "deployments"), "scale", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectMatches: false,
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

func Test_matchesResourceRules(t *testing.T) {
	tests := []struct {
		name               string
		operation          admission.Operation
		attrName           string
		object             runtime.Object
		resourceName       string
		allowGeneratedName bool
		expected           bool
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
			resourceName:       "test-prefix",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name:               "no match when object is nil and name is empty",
			object:             nil,
			resourceName:       "test-prefix",
			allowGeneratedName: true,
			expected:           false,
		},
		{
			name: "allow prefix match when allowGeneratedName is true (preserves HasPrefix for resourceRules)",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-malicious-pod-",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name: "reject prefix match when allowGeneratedName is false (excludeResourceRules bypass prevention)",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-malicious-pod-",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: false,
			expected:           false,
		},
		{
			name: "allow substring prefix when allowGeneratedName is true (preserves HasPrefix for resourceRules)",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "testing-app-",
					},
				},
			},
			resourceName:       "test",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name: "reject generatedName match when allowGeneratedName is false (e.g. exclusion rules)",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: false,
			expected:           false,
		},
		{
			name: "allow match when allowGeneratedName is true with exact delimiter prefix",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name: "reject when generateName is shorter than resourceName with trailing hyphen",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault",
					},
				},
			},
			resourceName:       "vault-",
			allowGeneratedName: true,
			expected:           false,
		},
		{
			name: "allow match when allowGeneratedName is true with trailing hyphen on both",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "vault-",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name: "allow match when allowGeneratedName is true with no trailing hyphen on either",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name: "allow multi-token prefix when allowGeneratedName is true (preserves HasPrefix for resourceRules)",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "app-prod-1-",
					},
				},
			},
			resourceName:       "app",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name: "reject empty generateName on empty attr name",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: true,
			expected:           false,
		},
		{
			name: "reject empty matched resourceName",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "",
			allowGeneratedName: true,
			expected:           false,
		},
		{
			name:               "direct name match on CREATE is accepted even if allowGeneratedName is false",
			operation:          admission.Create,
			attrName:           "vault-instance-1",
			object:             nil,
			resourceName:       "vault-instance-1",
			allowGeneratedName: false,
			expected:           true,
		},
		{
			name:               "direct name match on CREATE is accepted when allowGeneratedName is true",
			operation:          admission.Create,
			attrName:           "vault-instance-1",
			object:             nil,
			resourceName:       "vault-instance-1",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name:               "direct name mismatch on CREATE is rejected",
			operation:          admission.Create,
			attrName:           "vault-instance-1",
			object:             nil,
			resourceName:       "vault",
			allowGeneratedName: false,
			expected:           false,
		},
		{
			name:      "resolved name takes precedence over generateName on CREATE (matching resolved name)",
			operation: admission.Create,
			attrName:  "resolved-app",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "resolved-app",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name:      "resolved name controls matching on CREATE and non-matching resolved name rejects even if generateName matches",
			operation: admission.Create,
			attrName:  "other-app",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: true,
			expected:           false,
		},
		{
			name:               "direct name match on UPDATE is accepted when allowGeneratedName is false (exclusion rules)",
			operation:          admission.Update,
			attrName:           "vault-instance-1",
			object:             nil,
			resourceName:       "vault-instance-1",
			allowGeneratedName: false,
			expected:           true,
		},
		{
			name:               "direct name match on UPDATE is accepted when allowGeneratedName is true (selection rules)",
			operation:          admission.Update,
			attrName:           "vault-instance-1",
			object:             nil,
			resourceName:       "vault-instance-1",
			allowGeneratedName: true,
			expected:           true,
		},
		{
			name:               "direct name mismatch on UPDATE is rejected",
			operation:          admission.Update,
			attrName:           "vault-instance-1",
			object:             nil,
			resourceName:       "vault",
			allowGeneratedName: false,
			expected:           false,
		},
		{
			name:      "resolved name takes precedence over generateName on UPDATE (matching resolved name)",
			operation: admission.Update,
			attrName:  "vault-instance-1",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "unrelated-app-",
					},
				},
			},
			resourceName:       "vault-instance-1",
			allowGeneratedName: false,
			expected:           true,
		},
		{
			name:      "resolved name controls matching on UPDATE and generateName is ignored",
			operation: admission.Update,
			attrName:  "other-app",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: true,
			expected:           false,
		},
		{
			name:      "empty name on UPDATE does not match exclusion rule via generateName",
			operation: admission.Update,
			attrName:  "",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "vault-",
					},
				},
			},
			resourceName:       "vault",
			allowGeneratedName: false,
			expected:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := tt.operation
			if op == "" {
				op = admission.Create
			}

			attr := admission.NewAttributesRecord(
				tt.object,
				nil,
				schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				"default",   // namespace
				tt.attrName, // name (empty triggers generateName fallback)
				schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				"", // subresource
				op,
				nil,   // operationOptions
				false, // dryRun
				&user.DefaultInfo{Name: "test-user"},
			)

			namedRules := []v1.NamedRuleWithOperations{
				{
					RuleWithOperations: v1.RuleWithOperations{
						Operations: []v1.OperationType{v1.OperationType(op)},
						Rule: v1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Resources:   []string{"*/*"},
						},
					},
					ResourceNames: []string{tt.resourceName},
				},
			}

			match, err := matchesResourceRules(namedRules, attr, tt.allowGeneratedName)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, match)
		})
	}
}
