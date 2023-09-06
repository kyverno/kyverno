package values

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
)

type labels = map[string]string

type Variables struct {
	*valuesapi.Values
	Variables map[string]string
}

func (v Variables) HasVariables() bool {
	return len(v.Variables) == 0
}

func (v Variables) HasPolicyVariables(policy string) bool {
	if v.Values == nil {
		return false
	}
	for _, pol := range v.Values.Policies {
		if pol.Name == policy {
			return true
		}
	}
	return false
}

func (v Variables) Subresources() []valuesapi.Subresource {
	if v.Values == nil {
		return nil
	}
	return v.Values.Subresources
}

func (v Variables) NamespaceSelectors() map[string]labels {
	if v.Values == nil {
		return nil
	}
	out := map[string]labels{}
	if v.Values.NamespaceSelectors != nil {
		for _, n := range v.Values.NamespaceSelectors {
			out[n.Name] = n.Labels
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (v Variables) CheckVariableForPolicy(policy, resource, kind string, kindMap map[string]struct{}, variables ...string) (map[string]interface{}, error) {
	resourceValues := map[string]interface{}{}
	// first apply global values
	if v.Values != nil {
		for k, v := range v.GlobalValues {
			resourceValues[k] = v
		}
	}
	// apply resource values
	if v.Values != nil {
		for _, p := range v.Values.Policies {
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
	for k, v := range v.Variables {
		resourceValues[k] = v
	}
	// make sure `request.operation` is set
	if _, ok := resourceValues["request.operation"]; !ok {
		resourceValues["request.operation"] = "CREATE"
	}
	// skipping the variable check for non matching kind
	if _, ok := kindMap[kind]; ok {
		// TODO remove dependency to store
		if len(variables) > 0 && len(resourceValues) == 0 && store.HasPolicies() {
			return nil, fmt.Errorf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy, resource)
		}
	}
	return resourceValues, nil
}

func GetVariable(fs billy.Filesystem, resourcePath string, path string, vals *valuesapi.Values, vars ...string) (*Variables, error) {
	// if we already have values, skip the file
	if vals == nil && path != "" {
		v, err := Load(fs, filepath.Join(resourcePath, path))
		if err != nil {
			return nil, sanitizederror.NewWithError("unable to read yaml", fmt.Errorf("Unable to load variable file: %s (%w)", path, err))
		}
		vals = v
	}
	variables := Variables{
		Values:    vals,
		Variables: parseVariables(vars...),
	}

	storePolicies := []store.Policy{}
	if variables.Values != nil {
		for _, p := range variables.Values.Policies {
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
	return &variables, nil
}

// func GetVariables(vals *valuesapi.Values, vars ...string) Variables {
// 	variables := Variables{
// 		Variables: parseVariables(vars...),
// 	}
// 	if vals != nil {
// 		vals.GlobalValues = forceMapStringEntry(vals.GlobalValues, "request.operation", "CREATE")
// 		for _, p := range vals.Policies {
// 			for _, r := range p.Resources {
// 				r.Values = forceMapEntry(r.Values, "request.operation", "CREATE")
// 				// r.Values = removeRequestObject(r.Values)
// 			}
// 		}
// 		if vals.NamespaceSelectors != nil {
// 			variables.NamespaceSelectors = map[string]labels{}
// 			for _, n := range vals.NamespaceSelectors {
// 				variables.NamespaceSelectors[n.Name] = n.Labels
// 			}
// 		}
// 		variables.Subresources = vals.Subresources
// 	}
// 	return variables
// }

// func RemoveDuplicateAndObjectVariables(vars ...string) []string {
// 	clean := sets.New[string]()
// 	for _, v := range vars {
// 		if !strings.Contains(v, "request.object") && !strings.Contains(v, "element") && v == "elementIndex" {
// 			clean.Insert(v)
// 		}
// 	}
// 	return sets.List(clean)
// }

func NeedsVariables(vars ...string) bool {
	for _, v := range vars {
		if !strings.Contains(v, "request.object") && !strings.Contains(v, "element") && v == "elementIndex" {
			return true
		}
	}
	return false
}

func parseVariables(vars ...string) map[string]string {
	result := map[string]string{}
	for _, variable := range vars {
		variable = strings.TrimSpace(variable)
		kvs := strings.Split(variable, "=")
		if len(kvs) != 2 {
			// TODO warning
			continue
		}
		key := strings.TrimSpace(kvs[0])
		value := strings.TrimSpace(kvs[1])
		if len(value) == 0 || len(key) == 0 {
			// TODO log
			continue
		}
		if strings.Contains(key, "request.object.") {
			// TODO log
			continue
		}
		if result[key] == "" {
			// TODO log
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// func forceMapEntry(values map[string]interface{}, key string, value string) map[string]interface{} {
// 	// make sure we have a map to work with
// 	if values == nil {
// 		values = map[string]interface{}{}
// 	}
// 	if _, ok := values[key]; !ok {
// 		values[key] = value
// 	}
// 	return values
// }

// func forceMapStringEntry(values map[string]string, key string, value string) map[string]string {
// 	// make sure we have a map to work with
// 	if values == nil {
// 		values = map[string]string{}
// 	}
// 	if _, ok := values[key]; !ok {
// 		values[key] = value
// 	}
// 	return values
// }
