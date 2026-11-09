package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func podAttrs(namespace string, op admission.Operation) admission.Attributes {
	return admission.NewAttributesRecord(
		nil, nil,
		schema.GroupVersionKind{Version: "v1", Kind: "Pod"},
		namespace, "nginx",
		schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", op, nil, false, nil,
	)
}

func podRules(ops ...admissionregistrationv1.OperationType) []admissionregistrationv1.NamedRuleWithOperations {
	return []admissionregistrationv1.NamedRuleWithOperations{{
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Operations: ops,
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"pods"},
			},
		},
	}}
}

func labelledNamespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

// TestExplain checks Explain against the real matcher: for each case, Match decides the
// outcome and Explain is asked to describe it, so a drift between the two shows up here.
func TestExplain(t *testing.T) {
	tests := []struct {
		name        string
		constraints *admissionregistrationv1.MatchResources
		attr        admission.Attributes
		ns          runtime.Object
		wantMatch   bool
		wantReason  string
	}{
		{
			name:        "matches",
			constraints: &admissionregistrationv1.MatchResources{ResourceRules: podRules(admissionregistrationv1.Create)},
			attr:        podAttrs("prod", admission.Create),
			wantMatch:   true,
			wantReason:  "matched kind Pod, namespace prod, operation CREATE",
		},
		{
			name:        "operation not covered",
			constraints: &admissionregistrationv1.MatchResources{ResourceRules: podRules(admissionregistrationv1.Update)},
			attr:        podAttrs("prod", admission.Create),
			wantMatch:   false,
			wantReason:  "is not covered by the policy's resourceRules",
		},
		{
			name: "excluded",
			constraints: &admissionregistrationv1.MatchResources{
				ResourceRules:        podRules(admissionregistrationv1.Create),
				ExcludeResourceRules: podRules(admissionregistrationv1.Create),
			},
			attr:       podAttrs("prod", admission.Create),
			wantMatch:  false,
			wantReason: "excluded by the policy's excludeResourceRules",
		},
		{
			name: "object selector rejects",
			constraints: &admissionregistrationv1.MatchResources{
				ResourceRules:  podRules(admissionregistrationv1.Create),
				ObjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
			},
			attr:       podAttrs("prod", admission.Create),
			wantMatch:  false,
			wantReason: "objectSelector",
		},
		{
			name: "namespace selector rejects",
			constraints: &admissionregistrationv1.MatchResources{
				ResourceRules:     podRules(admissionregistrationv1.Create),
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
			},
			attr:       podAttrs("prod", admission.Create),
			ns:         labelledNamespace("prod", map[string]string{"env": "prod"}),
			wantMatch:  false,
			wantReason: `namespace "prod" does not satisfy the policy's namespaceSelector`,
		},
		{
			name: "namespace selector satisfied",
			constraints: &admissionregistrationv1.MatchResources{
				ResourceRules:     podRules(admissionregistrationv1.Create),
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			},
			attr:       podAttrs("prod", admission.Create),
			ns:         labelledNamespace("prod", map[string]string{"env": "prod"}),
			wantMatch:  true,
			wantReason: "matched kind Pod, namespace prod, operation CREATE",
		},
		{
			name:        "no constraints",
			constraints: nil,
			attr:        podAttrs("prod", admission.Create),
			wantMatch:   false,
			wantReason:  "no matchConstraints",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := false
			if tt.constraints != nil {
				var err error
				matched, err = NewMatcher().Match(&MatchCriteria{Constraints: tt.constraints}, tt.attr, tt.ns)
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantMatch, matched, "test setup: the real matcher disagrees with the case's expectation")
			assert.Contains(t, Explain(tt.constraints, tt.attr, tt.ns, matched), tt.wantReason)
		})
	}
}

func TestExplain_ClusterScoped(t *testing.T) {
	attr := podAttrs("", admission.Create)
	assert.Contains(t, Explain(&admissionregistrationv1.MatchResources{}, attr, nil, true), "cluster-scoped")
}

// TestDescribeRules checks that a rejected rule's description names the specific constraints
// that could have caused the rejection -- resourceNames, apiVersions and scope, not just
// apiGroups/resources/operations -- since a request can be rejected by any of them.
func TestDescribeRules(t *testing.T) {
	namespaced := admissionregistrationv1.NamespacedScope
	rules := []admissionregistrationv1.NamedRuleWithOperations{
		{
			ResourceNames: []string{"safe-pod"},
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
					Scope:       &namespaced,
				},
			},
		},
	}

	got := describeRules(rules)

	assert.Contains(t, got, `resourceNames=["safe-pod"]`, "a name-restricted rule must show which names it allows")
	assert.Contains(t, got, `apiVersions=["v1"]`)
	assert.Contains(t, got, "scope=Namespaced")
}

// TestDescribeRules_NoResourceNamesOmitsField keeps the common case (no name restriction)
// readable instead of always printing an empty resourceNames=[].
func TestDescribeRules_NoResourceNamesOmitsField(t *testing.T) {
	rules := []admissionregistrationv1.NamedRuleWithOperations{{
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}}

	got := describeRules(rules)

	assert.NotContains(t, got, "resourceNames=")
	assert.Contains(t, got, "scope=*", "an unset Scope defaults to *, per the Rule type's own doc comment")
}
