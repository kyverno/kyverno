package evaluator

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate_MatchConstraints(t *testing.T) {
	tests := []struct {
		name      string
		ivpol     *policiesv1beta1.ImageValidatingPolicy
		wantErr   bool
		wantField string
		wantMsg   string
	}{
		{
			name: "nil matchConstraints is rejected",
			ivpol: &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "nil-constraints"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchImageReferences: []policiesv1beta1.MatchImageReference{
						{Glob: "ghcr.io/*"},
					},
					Validations: []admissionregistrationv1.Validation{
						{Expression: "true"},
					},
				},
			},
			wantErr:   true,
			wantField: "spec.matchConstraints",
			wantMsg:   "a matchConstraints with at least one resource rule is required",
		},
		{
			name: "empty resourceRules is rejected",
			ivpol: &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-rules"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{},
					},
					MatchImageReferences: []policiesv1beta1.MatchImageReference{
						{Glob: "ghcr.io/*"},
					},
					Validations: []admissionregistrationv1.Validation{
						{Expression: "true"},
					},
				},
			},
			wantErr:   true,
			wantField: "spec.matchConstraints",
			wantMsg:   "a matchConstraints with at least one resource rule is required",
		},
		{
			name: "valid matchConstraints is accepted",
			ivpol: &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-constraints"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{""},
										APIVersions: []string{"v1"},
										Resources:   []string{"pods"},
									},
								},
							},
						},
					},
					MatchImageReferences: []policiesv1beta1.MatchImageReference{
						{Glob: "ghcr.io/*"},
					},
					Validations: []admissionregistrationv1.Validation{
						{Expression: "true"},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.ivpol, nil)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantField)
				assert.Contains(t, err.Error(), tt.wantMsg)
			} else {
				// The matchConstraints check should not produce an error.
				// Other compile errors may still be present, so we only
				// verify the matchConstraints error is absent.
				if err != nil {
					assert.NotContains(t, err.Error(), "matchConstraints")
				}
			}
		})
	}
}
