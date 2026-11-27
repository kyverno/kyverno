package autogen

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Test_Autogen_KindNameController covers #18833 for ImageValidatingPolicy,
// which shares autogen config resolution with ValidatingPolicy.
func Test_Autogen_KindNameController(t *testing.T) {
	t.Cleanup(func() { autogen.BareNameResolver = nil })
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
	autogen.BareNameResolver = autogen.RESTMapperBareNameResolver(mapper)

	targetResources := func(policyControllers ...string) []string {
		policy := ivpol.DeepCopy()
		policy.Spec.AutogenConfiguration.PodControllers.Controllers = policyControllers
		rules, err := Autogen(policy)
		assert.NoError(t, err)
		resources := []string{}
		for _, rule := range rules {
			for _, target := range rule.Targets {
				resources = append(resources, target.Resource)
			}
		}
		return resources
	}

	t.Run("plural resource name still generates the deployments rule", func(t *testing.T) {
		assert.Equal(t, []string{"deployments"}, targetResources("deployments"))
	})

	t.Run("kind name no longer generates a rule for a resource that cannot exist", func(t *testing.T) {
		// Admission does not validate controller names, so a Kind name like
		// this can be stored unchecked. Autogen resolves it through
		// ResolveConfig, whose bare-name fallback uses discovery where one is
		// available, and yields no rule for it.
		assert.Empty(t, targetResources("Deployment"))
	})
}
