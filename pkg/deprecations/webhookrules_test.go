package deprecations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

// servedVersions reads a CRD manifest under config/crds/kyverno and returns the set of versions
// it currently serves. This is the source of truth the webhook rule tables must never fall
// behind: a served version missing from a rule's APIVersions means the apiserver accepts legacy
// writes at that version without ever reaching the admission gate.
func servedVersions(t *testing.T, file string) sets.Set[string] {
	t.Helper()
	path := filepath.Join("..", "..", "config", "crds", "kyverno", file)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(raw, &crd))
	served := sets.New[string]()
	for _, v := range crd.Spec.Versions {
		if v.Served {
			served.Insert(v.Name)
		}
	}
	require.NotEmpty(t, served, "%s: no served versions found, CRD layout may have changed", file)
	return served
}

func TestLegacyPolicyRuleCoversServedVersions(t *testing.T) {
	served := servedVersions(t, "kyverno.io_clusterpolicies.yaml")
	rule := sets.New(LegacyPolicyRule.APIVersions...)
	assert.True(t, rule.IsSuperset(served), "LegacyPolicyRule.APIVersions %v does not cover served versions %v", LegacyPolicyRule.APIVersions, served.UnsortedList())
	assert.ElementsMatch(t, []string{"v1", "v2beta1"}, LegacyPolicyRule.APIVersions)
}

func TestLegacyPolicyStatusRuleCoversServedVersions(t *testing.T) {
	served := servedVersions(t, "kyverno.io_clusterpolicies.yaml")
	rule := sets.New(LegacyPolicyStatusRule.APIVersions...)
	assert.True(t, rule.IsSuperset(served), "LegacyPolicyStatusRule.APIVersions %v does not cover served versions %v", LegacyPolicyStatusRule.APIVersions, served.UnsortedList())
}

func TestLegacyExceptionRuleCoversServedVersions(t *testing.T) {
	served := servedVersions(t, "kyverno.io_policyexceptions.yaml")
	rule := sets.New(LegacyExceptionRule.APIVersions...)
	// v2alpha1 is kept in the rule for backwards compatibility even though no served CRD
	// version has ever used it; a harmless superset element is fine, a missing served
	// version is not.
	assert.True(t, rule.IsSuperset(served), "LegacyExceptionRule.APIVersions %v does not cover served versions %v", LegacyExceptionRule.APIVersions, served.UnsortedList())
	assert.True(t, rule.Has("v2"))
	assert.True(t, rule.Has("v2beta1"))
}

func TestLegacyCleanupPolicyRuleCoversServedVersions(t *testing.T) {
	for _, file := range []string{"kyverno.io_cleanuppolicies.yaml", "kyverno.io_clustercleanuppolicies.yaml"} {
		served := servedVersions(t, file)
		rule := sets.New(LegacyCleanupPolicyRule.APIVersions...)
		assert.True(t, rule.IsSuperset(served), "%s: LegacyCleanupPolicyRule.APIVersions %v does not cover served versions %v", file, LegacyCleanupPolicyRule.APIVersions, served.UnsortedList())
	}
	rule := sets.New(LegacyCleanupPolicyRule.APIVersions...)
	assert.True(t, rule.Has("v2"))
	assert.True(t, rule.Has("v2beta1"))
}
