package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func Test_validateActions_NilRule(t *testing.T) {
	warnings, err := validateActions(0, nil, nil, true, "", "")
	assert.Nil(t, err)
	assert.Nil(t, warnings)
}

func Test_validateActions_GenerateSameKind(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		MatchResources: kyvernov1.MatchResources{
			Any: kyvernov1.ResourceFilters{
				{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
					},
				},
			},
		},
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "ConfigMap",
				},
			},
		},
	}
	rule.MatchResources.Kinds = []string{"ConfigMap"}

	warnings, err := validateActions(0, rule, nil, true, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generation kind and match resource kind should not be the same")
	assert.Nil(t, warnings)
}

func Test_validateActions_GenerateMockSuccess(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "ConfigMap",
					Name: "test-cm",
				},
			},
		},
	}

	warnings, err := validateActions(0, rule, nil, true, "", "")
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

// Test_validateActions_GenerateOfflineSynchronize verifies that offline (mock) mode
// succeeds for a synchronize generate rule without any Kubernetes cluster access.
func Test_validateActions_GenerateOfflineSynchronize(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "NetworkPolicy",
					Name: "default-netpol",
				},
			},
			Synchronize: true,
		},
	}

	warnings, err := validateActions(0, rule, nil, true, "", "")
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

// Test_validateActions_GenerateOfflineStructuralError verifies that offline mode still
// rejects structurally invalid generate rules (e.g. CELPreconditions mixed with generate).
func Test_validateActions_GenerateOfflineStructuralError(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		CELPreconditions: []admissionregistrationv1.MatchCondition{
			{Name: "check", Expression: "true"},
		},
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "ConfigMap",
					Name: "test-cm",
				},
			},
		},
	}

	warnings, err := validateActions(0, rule, nil, true, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "celPrecondition can only be used with validate.cel")
	assert.Nil(t, warnings)
}
