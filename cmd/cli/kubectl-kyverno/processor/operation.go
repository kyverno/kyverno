package processor

import (
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
)

// NormalizeOperation validates a CLI-declared admission operation. An empty
// string is valid and means "use the default operation".
func NormalizeOperation(operation string) (string, error) {
	switch operation {
	case "", "CREATE", "UPDATE", "DELETE":
		return operation, nil
	default:
		return "", fmt.Errorf("invalid operation %q, must be one of CREATE, UPDATE, DELETE", operation)
	}
}

// NormalizeValuesOperation validates a `request.operation` global value from a
// values file. In addition to the operations supported by admission request
// simulation, CONNECT is accepted for backward compatibility with classic
// policies that evaluate `request.operation` in preconditions; it is not
// simulated for CEL policy types, which fall back to the CREATE request shape.
func NormalizeValuesOperation(operation string) (string, error) {
	switch operation {
	case "", "CREATE", "UPDATE", "DELETE", "CONNECT":
		return operation, nil
	default:
		return "", fmt.Errorf("invalid request.operation value %q, must be one of CREATE, UPDATE, DELETE, CONNECT", operation)
	}
}

// AdmissionRequestShape maps a CLI-declared operation to the admission operation
// and the (object, oldObject) pair, mirroring the API server semantics:
//   - CREATE (default): object is the resource, oldObject is null
//   - UPDATE: object and oldObject are both the resource
//   - DELETE: object is null, oldObject is the resource
//
// oldObjectOverride, when non-nil, is merged onto the default oldObject (the
// resource copy) for UPDATE and DELETE as an RFC 7386 JSON Merge Patch (see
// mergeOldObject), letting a values file describe the old object as a partial
// diff against the resource. It is ignored on CREATE, where there is no
// oldObject.
func AdmissionRequestShape(operation string, resource *unstructured.Unstructured, oldObjectOverride map[string]interface{}) (admissionv1.Operation, runtime.Object, runtime.Object) {
	switch operation {
	case "UPDATE":
		return admissionv1.Update, resource, mergeOldObject(resource, oldObjectOverride)
	case "DELETE":
		// deep copy so downstream mutations of the old state cannot leak into
		// the shared resource object
		return admissionv1.Delete, nil, mergeOldObject(resource, oldObjectOverride)
	default:
		return admissionv1.Create, resource, nil
	}
}

// mergeOldObject returns a deep copy of resource with override merged onto it
// as an RFC 7386 JSON Merge Patch: nested maps merge key by key, an explicit
// null deletes the key, and any other value replaces the leaf. A nil override
// returns a plain copy; a marshaling error logs and falls back to one too.
func mergeOldObject(resource *unstructured.Unstructured, override map[string]interface{}) *unstructured.Unstructured {
	old := resource.DeepCopy()
	if len(override) == 0 {
		return old
	}
	merged, err := mergeJSON(old.Object, override)
	if err != nil {
		log.Log.V(2).Info("failed to merge request.oldObject override, ignoring it", "error", err.Error())
		return old
	}
	old.Object = merged
	return old
}

// mergeJSON merge-patches override onto target (see mergeOldObject),
// marshaling through k8s.io/apimachinery/pkg/util/json so integers survive
// the round trip as int64 instead of encoding/json's default float64.
func mergeJSON(target, override map[string]interface{}) (map[string]interface{}, error) {
	targetJSON, err := utiljson.Marshal(target)
	if err != nil {
		return nil, err
	}
	overrideJSON, err := utiljson.Marshal(override)
	if err != nil {
		return nil, err
	}
	mergedJSON, err := jsonpatch.MergePatch(targetJSON, overrideJSON)
	if err != nil {
		return nil, err
	}
	var merged map[string]interface{}
	if err := utiljson.Unmarshal(mergedJSON, &merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// resolveOperation returns the effective operation for the processor: the
// explicitly configured operation takes precedence, then the `request.operation`
// global value from the values file, then the default (CREATE). CONNECT from the
// values file is not simulated and maps to the default request shape.
func (p *PolicyProcessor) resolveOperation() string {
	if p.Operation != "" {
		return p.Operation
	}
	if p.Variables != nil {
		if op, err := NormalizeOperation(p.Variables.GlobalOperation()); err == nil {
			return op
		}
	}
	return ""
}

// NamesOf returns the GetName() of each policy, for use with
// GroupByOldObjectOverride and Variables.OldObjectOverride.
func NamesOf[T interface{ GetName() string }](policies []T) []string {
	names := make([]string, 0, len(policies))
	for _, policy := range policies {
		names = append(names, policy.GetName())
	}
	return names
}

// namesPredicate returns a predicate matching any policy whose GetName() is
// in names, for splitting an engine Handle call by GroupByOldObjectOverride.
func namesPredicate[T interface{ GetName() string }](names []string) func(T) bool {
	set := sets.New(names...)
	return func(p T) bool { return set.Has(p.GetName()) }
}

// resolveOldObjectOverride resolves the request.oldObject override for one
// policy against one resource, or (nil, nil) if vars is nil or none is
// declared. See Variables.OldObjectOverride.
func resolveOldObjectOverride(vars *variables.Variables, policyName, resourceName, operation string) (map[string]interface{}, error) {
	if vars == nil || !hasOldObject(operation) {
		return nil, nil
	}
	return vars.OldObjectOverride([]string{policyName}, resourceName)
}

// OverrideGroup is a set of policy names that resolve to the same
// request.oldObject override for one resource.
type OverrideGroup struct {
	Override map[string]interface{}
	Names    []string
}

// GroupByOldObjectOverride resolves each of policyNames' request.oldObject
// override against resourceName and groups the names that resolve to the same
// override, in first-seen order, so a caller splitting one engine Handle call
// per group never leaks one policy's override onto another.
func GroupByOldObjectOverride(vars *variables.Variables, policyNames []string, resourceName, operation string) ([]OverrideGroup, error) {
	var groups []OverrideGroup
	index := map[string]int{}
	for _, name := range policyNames {
		override, err := resolveOldObjectOverride(vars, name, resourceName, operation)
		if err != nil {
			return nil, fmt.Errorf("policy %s: %w", name, err)
		}
		key, err := json.Marshal(override)
		if err != nil {
			return nil, err
		}
		if i, ok := index[string(key)]; ok {
			groups[i].Names = append(groups[i].Names, name)
			continue
		}
		index[string(key)] = len(groups)
		groups = append(groups, OverrideGroup{Override: override, Names: []string{name}})
	}
	return groups, nil
}

// mutatingOldObjectOverride resolves the one request.oldObject override that
// mutating policies share against resourceName: they chain patches in one
// request, so policies declaring none use it too, and different ones are an error.
func mutatingOldObjectOverride[T interface{ GetName() string }](vars *variables.Variables, policies []T, resourceName, operation string) (map[string]interface{}, error) {
	all, err := GroupByOldObjectOverride(vars, NamesOf(policies), resourceName, operation)
	if err != nil {
		return nil, err
	}
	var groups []OverrideGroup
	for _, g := range all {
		if g.Override != nil {
			groups = append(groups, g)
		}
	}
	if len(groups) > 1 {
		parts := make([]string, 0, len(groups))
		for _, g := range groups {
			parts = append(parts, fmt.Sprintf("%v", g.Names))
		}
		return nil, fmt.Errorf("mutating policies declare conflicting request.oldObject overrides for resource %q: %s (a single admission request can carry only one oldObject; split them across separate kyverno apply/test invocations)",
			resourceName, strings.Join(parts, " vs "))
	}
	if len(groups) == 0 {
		return nil, nil
	}
	return groups[0].Override, nil
}

// hasOldObject reports whether operation's admission request carries an oldObject.
func hasOldObject(operation string) bool {
	return operation == "UPDATE" || operation == "DELETE"
}
