package variables

import (
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Variables struct {
	values    *v1alpha1.ValuesSpec
	variables map[string]string
}

func (v Variables) Subresources() []v1alpha1.Subresource {
	if v.values == nil {
		return nil
	}
	if len(v.values.Subresources) == 0 {
		return nil
	}
	return v.values.Subresources
}

func (v Variables) NamespaceSelectors() map[string]Labels {
	if v.values == nil {
		return nil
	}
	out := map[string]Labels{}
	if v.values.NamespaceSelectors != nil {
		for _, n := range v.values.NamespaceSelectors {
			out[n.Name] = n.Labels
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (v Variables) ComputeVariables(policy, resource, kind string, kindMap sets.Set[string], variables ...string) (map[string]interface{}, error) {
	resourceValues := map[string]interface{}{}
	// first apply global values
	if v.values != nil {
		for k, v := range v.values.GlobalValues {
			resourceValues[k] = v
		}
	}
	// apply resource values
	if v.values != nil {
		for _, p := range v.values.Policies {
			if p.Name != policy {
				continue
			}
			for _, r := range p.Resources {
				if r.Name != resource {
					continue
				}
				for k, v := range r.Values {
					resourceValues[k] = v
				}
			}
		}
	}
	// apply variable
	for k, v := range v.variables {
		resourceValues[k] = v
	}
	// make sure `request.operation` is set
	if _, ok := resourceValues["request.operation"]; !ok {
		resourceValues["request.operation"] = "CREATE"
	}
	// skipping the variable check for non matching kind
	// TODO remove dependency to store
	if kindMap.Has(kind) && len(variables) > 0 && len(resourceValues) == 0 && store.HasPolicies() {
		return nil, fmt.Errorf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy, resource)
	}
	return resourceValues, nil
}

func (v Variables) SetInStore() {
	storePolicies := []store.Policy{}
	if v.values != nil {
		for _, p := range v.values.Policies {
			sp := store.Policy{
				Name: p.Name,
			}
			for _, r := range p.Rules {
				sr := store.Rule{
					Name:          r.Name,
					Values:        r.Values,
					ForEachValues: r.ForeachValues,
				}
				sp.Rules = append(sp.Rules, sr)
			}
			storePolicies = append(storePolicies, sp)
		}
	}
	store.SetPolicies(storePolicies...)
}
