package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// TestExplain checks Explain against the real matcher: for each case, Match decides the
// outcome and Explain is asked to describe it, so a drift between the two shows up here.
func TestExplain(t *testing.T) {
	tests := []struct {
		name        string
		constraints *admissionregistrationv1.MatchResources
		attr        admission.Attributes
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
				matched, err = NewMatcher().Match(&MatchCriteria{Constraints: tt.constraints}, tt.attr, nil)
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantMatch, matched, "test setup: the real matcher disagrees with the case's expectation")
			assert.Contains(t, Explain(tt.constraints, tt.attr, nil, matched), tt.wantReason)
		})
	}
}

func TestExplain_ClusterScoped(t *testing.T) {
	attr := podAttrs("", admission.Create)
	assert.Contains(t, Explain(&admissionregistrationv1.MatchResources{}, attr, nil, true), "cluster-scoped")
}
