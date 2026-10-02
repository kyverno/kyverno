package policy

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/policy/auth"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Test_validateActions_NilRule verifies a nil rule is a no-op.
func Test_validateActions_NilRule(t *testing.T) {
	warnings, err := validateActions(0, nil, nil, true, "", "", nil)
	assert.Nil(t, err)
	assert.Nil(t, warnings)
}

// Test_validateActions_GenerateSameKind verifies a generate rule whose target
// kind matches its own match kind is rejected.
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

	warnings, err := validateActions(0, rule, nil, true, "", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generation kind and match resource kind should not be the same")
	assert.Nil(t, warnings)
}

// Test_validateActions_GenerateMockSuccess verifies a valid generate rule
// passes in mock mode.
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

	warnings, err := validateActions(0, rule, nil, true, "", "", nil)
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

// Test_validateActions_LiveAuthError exercises the non-mock caller: a real
// dclient whose SubjectAccessReview requests fail. It drives validateActions
// through the live (reportsSA, non-nil cache) path and asserts the
// authorization error is surfaced rather than swallowed. The cache tests cover
// the decorator in isolation; this covers the caller wiring.
func Test_validateActions_LiveAuthError(t *testing.T) {
	kube := kubefake.NewSimpleClientset()
	kube.PrependReactor("create", "subjectaccessreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("authorization request failed")
	})
	client := dclient.NewFakeClientWithDisco(
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		kube,
		dclient.NewFakeDiscoveryClient(nil),
	)

	rule := &kyvernov1.Rule{
		Name: "deny-rule",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"ConfigMap"},
			},
		},
		Validation: &kyvernov1.Validation{
			Deny: &kyvernov1.Deny{},
		},
	}

	_, err := validateActions(0, rule, client, false, "background-sa", "reports-sa", auth.NewResultCache())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorization request failed")
}
