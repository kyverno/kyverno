package variables

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
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
	for _, n := range v.values.NamespaceSelectors {
		out[n.Name] = n.Labels
	}
	// Give precedence to namespaces
	for _, n := range v.values.Namespaces {
		out[n.Name] = n.Labels
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (v Variables) Namespace(name string) *corev1.Namespace {
	if v.values == nil {
		return nil
	}
	// Give precedence to namespaces
	for _, n := range v.values.Namespaces {
		if n.Name == name {
			return &n
		}
	}
	for _, n := range v.values.NamespaceSelectors {
		if n.Name == name {
			return &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Namespace",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: n.Labels,
				},
			}
		}
	}
	return nil
}

// GlobalOperation returns the admission operation declared via the
// `request.operation` global value, or an empty string if unset.
func (v Variables) GlobalOperation() string {
	if v.values == nil {
		return ""
	}
	if op, ok := v.values.GlobalValues["request.operation"]; ok {
		if s, ok := op.(string); ok {
			return s
		}
	}
	return ""
}

func (v Variables) ComputeVariables(s *store.Store, policy, resource, kind string, kindMap sets.Set[string], variables ...string) (map[string]interface{}, error) {
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
	if kindMap.Has(kind) && len(variables) > 0 && len(resourceValues) == 0 && s.HasPolicies() {
		return nil, fmt.Errorf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy, resource)
	}
	return resourceValues, nil
}

func (v Variables) SetInStore(s *store.Store) {
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
	s.SetPolicies(storePolicies...)
}

func mergeValuesMap(dst, src map[string]interface{}) {
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	// Sorted order ensures deterministic evaluation and guarantees canonical "request.oldObject"
	// overrides "oldObject" aliases if both are present in the same map.
	sort.Strings(keys)

	// If src provides a full request.oldObject or oldObject, clear any existing dotted
	// overrides from earlier layers so they do not inadvertently overwrite the new full object.
	hasFullOldObject := false
	for _, k := range keys {
		if k == "request.oldObject" || k == "oldObject" {
			hasFullOldObject = true
			break
		} else if k == "request" {
			if reqMap, ok := cleanMapKeys(src[k]).(map[string]interface{}); ok {
				if _, ok := reqMap["oldObject"]; ok {
					hasFullOldObject = true
					break
				}
			}
		}
	}
	if hasFullOldObject {
		for dk := range dst {
			if strings.HasPrefix(dk, "request.oldObject.") || strings.HasPrefix(dk, "oldObject.") {
				delete(dst, dk)
			}
		}
	}

	for _, rawK := range keys {
		k := rawK
		v := cleanMapKeys(src[rawK])
		// Normalize oldObject aliases to canonical "request.oldObject"
		if k == "oldObject" {
			k = "request.oldObject"
		} else if strings.HasPrefix(k, "oldObject.") {
			k = "request.oldObject." + strings.TrimPrefix(k, "oldObject.")
		} else if k == "request" {
			if reqMap, ok := v.(map[string]interface{}); ok {
				reqKeys := make([]string, 0, len(reqMap))
				for rk := range reqMap {
					reqKeys = append(reqKeys, rk)
				}
				sort.Strings(reqKeys)
				for _, reqK := range reqKeys {
					reqV := reqMap[reqK]
					if reqK == "oldObject" {
						dst["request.oldObject"] = cleanMapKeys(reqV)
					} else if strings.HasPrefix(reqK, "oldObject.") {
						dst["request.oldObject."+strings.TrimPrefix(reqK, "oldObject.")] = cleanMapKeys(reqV)
					}
				}
			}
		}

		if k == "request.oldObject" {
			delete(dst, "oldObject")
			dst[k] = v
			continue
		}

		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				mergeValuesMap(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}

// OldObject returns the oldObject specified in the values file for the given resource and policy.
// It merges applicable values following the precedence order:
//  1. Global values from values.yaml
//  2. Policy-specific and resource-matching values from values.yaml
//  3. CLI variables (e.g. `--set`)
//
// Partial dot-notation overrides (e.g. `request.oldObject.metadata.labels.foo: bar`)
// are applied on top of the merged full oldObject (or on top of base if no full object is supplied).
// If no oldObject or oldObject dot-notation override is configured, it returns nil.
func (v Variables) OldObject(base *unstructured.Unstructured, resourceName string, policyName ...string) *unstructured.Unstructured {
	var targetPolicy string
	if len(policyName) > 0 {
		targetPolicy = policyName[0]
	}

	mergedValues := make(map[string]interface{})

	// 1. Global values
	if v.values != nil && v.values.GlobalValues != nil {
		mergeValuesMap(mergedValues, v.values.GlobalValues)
	}

	// 2. Resource values from matching policies
	if v.values != nil {
		// First apply wildcard policies if any
		for _, p := range v.values.Policies {
			if p.Name == "*" || p.Name == "" {
				for _, r := range p.Resources {
					if matchResource(r.Name, resourceName, base) {
						mergeValuesMap(mergedValues, r.Values)
					}
				}
			}
		}
		// Then apply specific policy if targeted
		if targetPolicy != "" && targetPolicy != "*" {
			for _, p := range v.values.Policies {
				if p.Name == targetPolicy {
					for _, r := range p.Resources {
						if matchResource(r.Name, resourceName, base) {
							mergeValuesMap(mergedValues, r.Values)
						}
					}
				}
			}
		}
	}

	// 3. CLI variables
	if len(v.variables) > 0 {
		varMap := make(map[string]interface{}, len(v.variables))
		for k, val := range v.variables {
			varMap[k] = val
		}
		mergeValuesMap(mergedValues, varMap)
	}

	return extractOldObject(mergedValues, base)
}

func matchResource(rName string, resName string, resource *unstructured.Unstructured) bool {
	if rName == resName {
		return true
	}
	if resource == nil {
		return false
	}
	if rName == resource.GetName() {
		return true
	}
	if ns := resource.GetNamespace(); ns != "" {
		if rName == ns+"/"+resource.GetName() {
			return true
		}
	}
	if kind := resource.GetKind(); kind != "" {
		if rName == kind+"/"+resource.GetName() {
			return true
		}
		if ns := resource.GetNamespace(); ns != "" {
			if rName == kind+"/"+ns+"/"+resource.GetName() {
				return true
			}
		}
	}
	return false
}

func extractOldObject(values map[string]interface{}, base *unstructured.Unstructured) *unstructured.Unstructured {
	if len(values) == 0 {
		return nil
	}
	var rawObj any
	if v, ok := values["request.oldObject"]; ok {
		rawObj = v
	} else if v, ok := values["oldObject"]; ok {
		rawObj = v
	} else if req, ok := values["request"].(map[string]interface{}); ok {
		if v, ok := req["oldObject"]; ok {
			rawObj = v
		}
	}

	var target *unstructured.Unstructured
	if rawObj != nil {
		target = toUnstructured(rawObj)
	}

	var dotKeys []struct {
		path []string
		val  interface{}
	}
	for k, v := range values {
		var sub string
		if strings.HasPrefix(k, "request.oldObject.") {
			sub = strings.TrimPrefix(k, "request.oldObject.")
		} else if strings.HasPrefix(k, "oldObject.") {
			sub = strings.TrimPrefix(k, "oldObject.")
		}
		if sub != "" {
			parts := strings.Split(sub, ".")
			dotKeys = append(dotKeys, struct {
				path []string
				val  interface{}
			}{path: parts, val: v})
		}
	}

	if len(dotKeys) > 0 {
		sort.Slice(dotKeys, func(i, j int) bool {
			return len(dotKeys[i].path) < len(dotKeys[j].path)
		})
		if target == nil {
			if base != nil {
				target = base.DeepCopy()
			} else {
				target = &unstructured.Unstructured{Object: make(map[string]interface{})}
			}
		}
		for _, dk := range dotKeys {
			_ = unstructured.SetNestedField(target.Object, dk.val, dk.path...)
		}
	}

	return target
}

func cleanMapKeys(val any) any {
	switch v := val.(type) {
	case map[string]interface{}:
		res := make(map[string]interface{}, len(v))
		for k, val := range v {
			res[k] = cleanMapKeys(val)
		}
		return res
	case map[interface{}]interface{}:
		res := make(map[string]interface{}, len(v))
		for k, val := range v {
			res[fmt.Sprintf("%v", k)] = cleanMapKeys(val)
		}
		return res
	case []interface{}:
		res := make([]interface{}, len(v))
		for i, val := range v {
			res[i] = cleanMapKeys(val)
		}
		return res
	default:
		return v
	}
}

func toUnstructured(val any) *unstructured.Unstructured {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case *unstructured.Unstructured:
		if v == nil {
			return nil
		}
		return v.DeepCopy()
	case unstructured.Unstructured:
		return v.DeepCopy()
	case string:
		var m map[string]interface{}
		if err := yaml.Unmarshal([]byte(v), &m); err == nil && len(m) > 0 {
			return &unstructured.Unstructured{Object: m}
		}
		return nil
	default:
		cleaned := cleanMapKeys(val)
		if m, ok := cleaned.(map[string]interface{}); ok && len(m) > 0 {
			return (&unstructured.Unstructured{Object: m}).DeepCopy()
		}
		b, err := json.Marshal(val)
		if err != nil {
			return nil
		}
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil || len(m) == 0 {
			return nil
		}
		return &unstructured.Unstructured{Object: m}
	}
}
