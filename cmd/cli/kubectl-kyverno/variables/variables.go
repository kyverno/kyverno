package variables

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// oldObjectKey is the values-file key that carries the request.oldObject
// override, either as a single map (oldObjectKey) or folded in dotted form
// (oldObjectKey + "." + <path>), e.g. request.oldObject.metadata.labels.x.
const oldObjectKey = "request.oldObject"

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

// OldObjectOverride resolves the request.oldObject override declared in the
// values file for resourceName, scoped to one of policyNames, folding in any
// dotted request.oldObject.<path> keys. It returns (nil, nil) if none is
// declared, and an error if a declared request.oldObject isn't a map.
func (v Variables) OldObjectOverride(policyNames []string, resourceName string) (map[string]interface{}, error) {
	if v.values == nil {
		return nil, nil
	}
	flat := map[string]interface{}{}
	for k, val := range v.values.GlobalValues {
		if isOldObjectKey(k) {
			flat[k] = val
		}
	}
	wanted := sets.New(policyNames...)
	for _, p := range v.values.Policies {
		if !wanted.Has(p.Name) {
			continue
		}
		for _, r := range p.Resources {
			if r.Name != resourceName {
				continue
			}
			for k, val := range r.Values {
				if isOldObjectKey(k) {
					flat[k] = val
				}
			}
		}
	}
	if len(flat) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	override := map[string]interface{}{}
	for _, k := range keys {
		val := flat[k]
		if k == oldObjectKey {
			m, ok := val.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("%s for resource %q must be a map, got %T", oldObjectKey, resourceName, val)
			}
			foldOverride(override, m)
			continue
		}
		if err := setDottedPath(override, strings.TrimPrefix(k, oldObjectKey+"."), val); err != nil {
			return nil, err
		}
	}
	return override, nil
}

func isOldObjectKey(k string) bool {
	return k == oldObjectKey || strings.HasPrefix(k, oldObjectKey+".")
}

// foldOverride deep-merges src into dest: nested maps merge key by key, and
// everything else, including an explicit null, replaces the dest entry
// as-is, for mergeOldObject to interpret as a delete later.
func foldOverride(dest, src map[string]interface{}) {
	for k, v := range src {
		if m, ok := v.(map[string]interface{}); ok {
			if dm, ok := dest[k].(map[string]interface{}); ok {
				foldOverride(dm, m)
				continue
			}
			// A fresh map, so a later dotted key never writes into the values file's own map.
			fresh := map[string]interface{}{}
			foldOverride(fresh, m)
			dest[k] = fresh
			continue
		}
		dest[k] = v
	}
}

// setDottedPath sets dest[p1][p2]...[pn] = val, creating intermediate maps as
// needed, so a dotted request.oldObject.<path> key folds into the same shape
// as the map form of request.oldObject.
func setDottedPath(dest map[string]interface{}, dotted string, val interface{}) error {
	// Split like the 1.19 JSON context did, so a quoted segment such as "app.example.com/tier" keeps its dots.
	reader := csv.NewReader(strings.NewReader(dotted))
	reader.Comma = '.'
	parts, err := reader.Read()
	if err != nil {
		return fmt.Errorf("invalid key %s.%s: %w", oldObjectKey, dotted, err)
	}
	cur := dest
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cur[part] = next
		}
		cur = next
	}
	if m, ok := val.(map[string]interface{}); ok {
		// A fresh map, as in foldOverride, so a deeper dotted key never writes into the values file's map.
		fresh := map[string]interface{}{}
		foldOverride(fresh, m)
		val = fresh
	}
	cur[parts[len(parts)-1]] = val
	return nil
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
