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

// To add a name/exclude case to TestMatcher, use:
//
//	matchResources(include, exclude)  // rules to include and exclude, nil for none
//	deployRule("name")                // apps/v1 deployments rule limited to names; no names = any name
//	deployAttrs(op, name, obj)        // the request; obj is nil unless testing generateName
var allOps = []v1.OperationType{v1.OperationAll}

func namedRule(ops []v1.OperationType, group, version, resource string, names ...string) v1.NamedRuleWithOperations {
	return v1.NamedRuleWithOperations{
		ResourceNames: names,
		RuleWithOperations: v1.RuleWithOperations{
			Operations: ops,
			Rule:       v1.Rule{APIGroups: []string{group}, APIVersions: []string{version}, Resources: []string{resource}},
		},
	}
}

// deployRule matches apps/v1 deployments for all operations, optionally restricted to names.
func deployRule(names ...string) v1.NamedRuleWithOperations {
	return namedRule(allOps, "apps", "v1", "deployments", names...)
}

func matchResources(include, exclude []v1.NamedRuleWithOperations) *v1.MatchResources {
	return &v1.MatchResources{
		NamespaceSelector:    &metav1.LabelSelector{},
		ObjectSelector:       &metav1.LabelSelector{},
		ResourceRules:        include,
		ExcludeResourceRules: exclude,
	}
}

func deployAttrs(op admission.Operation, name string, obj runtime.Object) admission.Attributes {
	return admission.NewAttributesRecord(obj, nil, gvk("apps", "v1", "Deployment"), "ns", name, gvr("apps", "v1", "deployments"), "", op, &metav1.CreateOptions{}, false, nil)
}

func generateNameObject(generateName string) runtime.Object {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"generateName": generateName},
	}}
}

func TestMatcher(t *testing.T) {
	a := NewMatcher()

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
			name:          "name match, operation matches",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule([]v1.OperationType{v1.Create}, "apps", "v1", "deployments", "name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "name match, operation does not match",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule([]v1.OperationType{v1.Create}, "apps", "v1", "deployments", "name")}, nil),
			attrs:         deployAttrs(admission.Update, "name", nil),
			expectMatches: false,
		},
		{
			name:          "name match, resource version does not match",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "apps", "v1beta1", "deployments", "name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "multiple names in one rule, last name matches",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("a", "b", "name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "multiple names in one rule, no name matches",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("a", "b", "c")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "multiple rules, only the second rule matches the name",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("a"), deployRule("name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "multiple rules, name only listed in a rule for another version",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "apps", "v1beta1", "deployments", "name"), deployRule("other")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "exclude by name, name is excluded",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{deployRule("name")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "exclude by name, name is not excluded",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{deployRule("other")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "exclude by multiple names, second name is excluded",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{deployRule("a", "name")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "exclude by multiple names, no name is excluded",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{deployRule("a", "b")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "exclude with empty names excludes every name",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("name")}, []v1.NamedRuleWithOperations{deployRule()}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "exclude by name, operation does not match the exclude rule",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{namedRule([]v1.OperationType{v1.Delete}, "apps", "v1", "deployments", "name")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "include and exclude the same name, exclude wins",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("name")}, []v1.NamedRuleWithOperations{deployRule("name")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "include and exclude different names, included name matches",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("name")}, []v1.NamedRuleWithOperations{deployRule("other")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "name is excluded but not included",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("other")}, []v1.NamedRuleWithOperations{deployRule("name")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "empty name, generateName has the rule name as prefix",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("name-")}, nil),
			attrs:         deployAttrs(admission.Create, "", generateNameObject("name-")),
			expectMatches: true,
		},
		{
			name:          "empty name, generateName does not have the rule name as prefix",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("name-")}, nil),
			attrs:         deployAttrs(admission.Create, "", generateNameObject("other-")),
			expectMatches: false,
		},
		{
			name:          "empty name and empty generateName",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("name-")}, nil),
			attrs:         deployAttrs(admission.Create, "", generateNameObject("")),
			expectMatches: false,
		},
		{
			name:          "non-empty name ignores generateName",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("name-")}, nil),
			attrs:         deployAttrs(admission.Create, "other", generateNameObject("name-")),
			expectMatches: false,
		},
		{
			name:          "exclude by name, empty name excluded through generateName prefix",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{deployRule("tmp-")}),
			attrs:         deployAttrs(admission.Create, "", generateNameObject("tmp-")),
			expectMatches: false,
		},
		{
			name:          "empty names in include rule matches any name",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, nil),
			attrs:         deployAttrs(admission.Create, "any-name", nil),
			expectMatches: true,
		},
		{
			name:          "empty names in include rule matches empty name",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, nil),
			attrs:         deployAttrs(admission.Create, "", nil),
			expectMatches: true,
		},
		{
			name:          "name match, api group does not match",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "extensions", "v1", "deployments", "name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "name match, resource does not match",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "apps", "v1", "statefulsets", "name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "name match, wildcard group, version and resource",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "*", "*", "*", "name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "name miss, wildcard group, version and resource",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "*", "*", "*", "other")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "name match, rule lists several operations",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule([]v1.OperationType{v1.Create, v1.Update}, "apps", "v1", "deployments", "name")}, nil),
			attrs:         deployAttrs(admission.Update, "name", nil),
			expectMatches: true,
		},
		{
			name:          "name match, operation not among several listed operations",
			criteria:      matchResources([]v1.NamedRuleWithOperations{namedRule([]v1.OperationType{v1.Create, v1.Update}, "apps", "v1", "deployments", "name")}, nil),
			attrs:         deployAttrs(admission.Delete, "name", nil),
			expectMatches: false,
		},
		{
			name:          "name matching is case sensitive",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule("Name")}, nil),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:     "name match on subresource request",
			criteria: matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "apps", "v1", "deployments/scale", "name")}, nil),
			attrs: admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "scale",
				admission.Update, &metav1.UpdateOptions{}, false, nil),
			expectMatches: true,
		},
		{
			name:     "name miss on subresource request",
			criteria: matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "apps", "v1", "deployments/scale", "other")}, nil),
			attrs: admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "scale",
				admission.Update, &metav1.UpdateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name:     "exclude by name on subresource request",
			criteria: matchResources([]v1.NamedRuleWithOperations{namedRule(allOps, "apps", "v1", "deployments/scale")}, []v1.NamedRuleWithOperations{namedRule(allOps, "apps", "v1", "deployments/scale", "name")}),
			attrs: admission.NewAttributesRecord(nil, nil, gvk("autoscaling", "v1", "Scale"), "ns", "name", gvr("apps", "v1", "deployments"), "scale",
				admission.Update, &metav1.UpdateOptions{}, false, nil),
			expectMatches: false,
		},
		{
			name:          "exclude by name, operation matches the exclude rule",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{namedRule([]v1.OperationType{v1.Delete}, "apps", "v1", "deployments", "name")}),
			attrs:         deployAttrs(admission.Delete, "name", nil),
			expectMatches: false,
		},
		{
			name:          "exclude by name, api group does not match the exclude rule",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{namedRule(allOps, "extensions", "v1", "deployments", "name")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: true,
		},
		{
			name:          "multiple exclude rules, only the second excludes the name",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{deployRule("a"), deployRule("name")}),
			attrs:         deployAttrs(admission.Create, "name", nil),
			expectMatches: false,
		},
		{
			name:          "exclude only applies to the named resource",
			criteria:      matchResources([]v1.NamedRuleWithOperations{deployRule()}, []v1.NamedRuleWithOperations{deployRule("name")}),
			attrs:         deployAttrs(admission.Create, "other", nil),
			expectMatches: true,
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

			match, err := matchesResourceRules(namedRules, attr)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, match)
		})
	}
}
