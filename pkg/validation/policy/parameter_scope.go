package policy

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// validateCELParamKindScope resolves the requested parameter version directly:
// preferred-resource discovery does not include every served version of a kind.
func validateCELParamKindScope(rule kyvernov1.Rule, namespaced bool, client dclient.Interface) error {
	if !namespaced || !rule.HasValidateCEL() || !rule.Validation.CEL.HasParam() {
		return nil
	}

	paramKind := rule.Validation.CEL.ParamKind
	gv, err := schema.ParseGroupVersion(paramKind.APIVersion)
	if err != nil || gv.Version == "" || gv.String() != paramKind.APIVersion {
		return fmt.Errorf("invalid paramKind.apiVersion %q", paramKind.APIVersion)
	}
	if paramKind.Kind == "" {
		return fmt.Errorf("paramKind.kind must not be empty")
	}
	if client == nil || client.Discovery() == nil || client.Discovery().CachedDiscoveryInterface() == nil {
		return fmt.Errorf("cannot resolve paramKind %s/%s: discovery client is unavailable", paramKind.APIVersion, paramKind.Kind)
	}

	cachedDiscovery := client.Discovery().CachedDiscoveryInterface()
	var scopeErr error
	for attempt := 0; attempt < 2; attempt++ {
		resources, err := cachedDiscovery.ServerResourcesForGroupVersion(paramKind.APIVersion)
		if err != nil {
			scopeErr = fmt.Errorf("failed to resolve paramKind %s/%s: %w", paramKind.APIVersion, paramKind.Kind, err)
		} else {
			matches := 0
			if resources != nil && resources.GroupVersion == paramKind.APIVersion {
				for _, resource := range resources.APIResources {
					if resource.Kind != paramKind.Kind || strings.Contains(resource.Name, "/") {
						continue
					}
					if (resource.Group != "" && resource.Group != gv.Group) || (resource.Version != "" && resource.Version != gv.Version) {
						continue
					}
					if !resource.Namespaced {
						return fmt.Errorf("cluster-scoped paramKind is not allowed in namespaced policies")
					}
					matches++
				}
			}
			if matches == 1 {
				return nil
			}
			scopeErr = fmt.Errorf("cannot resolve paramKind %s/%s to a single namespaced resource", paramKind.APIVersion, paramKind.Kind)
		}
		if attempt == 0 {
			// Refresh discovery once so a newly installed CRD can be resolved.
			cachedDiscovery.Invalidate()
		}
	}
	return scopeErr
}
