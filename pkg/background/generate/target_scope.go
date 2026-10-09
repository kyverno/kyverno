package generate

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// validateTargetScope checks the substituted target even for policies stored
// before admission required static scope fields. It also runs for each foreach
// element, before any generation or clone source access.
func (g *generator) validateTargetScope(pattern *kyvernov1.GeneratePattern) error {
	if !g.policy.IsNamespaced() {
		return nil
	}
	policyNamespace := g.policy.GetNamespace()
	if policyNamespace == "" {
		return fmt.Errorf("generate requires a policy namespace")
	}
	if pattern.Namespace != policyNamespace {
		return fmt.Errorf("generate.namespace %q must equal policy namespace %q", pattern.Namespace, policyNamespace)
	}

	// Match generation dispatch: a single clone takes precedence over cloneList.
	if pattern.Clone.Name == "" && len(pattern.CloneList.Kinds) != 0 {
		for _, kind := range pattern.CloneList.Kinds {
			apiVersion, kind := kubeutils.GetKindFromGVK(kind)
			if err := g.validateNamespacedKind("generate.cloneList", "target", kyvernov1.ResourceSpec{APIVersion: apiVersion, Kind: kind}); err != nil {
				return err
			}
		}
		return nil
	}
	return g.validateNamespacedKind("generate", "target", pattern.ResourceSpec)
}

func (g *generator) validateNamespacedKind(path, role string, resource kyvernov1.ResourceSpec) error {
	kind, subresource := kubeutils.SplitSubresource(resource.Kind)
	if kind == "" || subresource != "" || wildcard.ContainsWildcard(resource.APIVersion+"/"+kind) {
		return fmt.Errorf("%s requires a namespaced top-level %s kind", path, role)
	}
	gv, err := schema.ParseGroupVersion(resource.APIVersion)
	if err != nil {
		return fmt.Errorf("%s %s apiVersion: %w", path, role, err)
	}
	if resource.APIVersion == "" {
		// Generation can resolve an omitted apiVersion through discovery. Require
		// every matching kind to be namespaced before allowing that lookup.
		gv.Group, gv.Version = "*", "*"
	}
	resources, err := g.client.Discovery().FindResources(gv.Group, gv.Version, kind, "")
	if err != nil {
		return fmt.Errorf("%s %s scope: %w", path, role, err)
	}
	if len(resources) == 0 {
		return fmt.Errorf("%s cannot determine %s scope for %s/%s", path, role, resource.APIVersion, kind)
	}
	for _, discovered := range resources {
		if !discovered.Namespaced {
			return fmt.Errorf("%s %s %s/%s must be namespaced", path, role, resource.APIVersion, kind)
		}
	}
	return nil
}
