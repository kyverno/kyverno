package admissionpolicy

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

// TestLegacyPathDynCostBlindness reproduces the reviewer's finding through the real
// admissionpolicy.NewCompiler -> validating.NewValidator.Validate path (validateResource,
// this package's own entry point for VAP admission), not through BaseEnvSet directly --
// BaseEnvSet alone is exactly what hid the bug, since the env it returns is correct and
// only loses CostLib's Estimator further down cel.NewCompositedCompiler's own chain
// (environment.StrictCostOpt's cel.CostTracking overwrite, costlib.go's doc comment).
//
// Shape: the same dyn string-concat amplification as
// TestRegressionShape1StringConcatAmplification in pkg/cel/compiler, reached through a
// ValidatingAdmissionPolicy's spec.validations instead of a vpol/mpol expression. Before
// LegacyDynCostFallback, this shape allocated over 1 GB and was silently admitted.
func TestLegacyPathDynCostBlindness(t *testing.T) {
	blob := strings.Repeat("b", 400_000)
	annotations := map[string]string{"blob": blob}
	for i := 0; i < 50; i++ {
		annotations["k"+string(rune('a'+i%26))+string(rune('0'+i/26))] = "x"
	}

	rawResource, err := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":        "test-pod",
			"namespace":   "default",
			"annotations": annotations,
		},
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "nginx", "image": "nginx:latest"}},
		},
	})
	assert.NilError(t, err)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "vap-dyn-cost"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{{
				Expression: `object.metadata.annotations.map(k, object.metadata.annotations["blob"] + object.metadata.annotations["blob"]).map(s, s + s).size() >= 0`,
			}},
		},
	}

	gvk := resource.GroupVersionKind()
	restMapper, err := utils.GetRESTMapper(nil)
	assert.NilError(t, err)
	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	assert.NilError(t, err)
	gvr := mapping.Resource

	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: resource.GetNamespace()}}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	response, err := validateResource(policy, nil, *resource, nil, ns, a)
	assert.NilError(t, err)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocDelta := after.TotalAlloc - before.TotalAlloc

	if len(response.PolicyResponse.Rules) != 1 {
		t.Fatalf("expected exactly one rule response, got %d", len(response.PolicyResponse.Rules))
	}
	rule := response.PolicyResponse.Rules[0]
	if rule.Status() != "error" && rule.Status() != "fail" {
		t.Fatalf("expected the legacy path to refuse this shape (status error/fail), got status=%q message=%q, allocDelta=%d bytes (%.1f MB) -- was >1 GB, silently admitted, before LegacyDynCostFallback",
			rule.Status(), rule.Message(), allocDelta, float64(allocDelta)/(1<<20))
	}
	if allocDelta > 64<<20 {
		t.Fatalf("allocation = %d bytes (%.1f MB), want < 64 MB (was >1,151 MB, admitted, before LegacyDynCostFallback)", allocDelta, float64(allocDelta)/(1<<20))
	}
	t.Logf("legacy path dyn cost blindness: status=%q message=%q allocDelta=%d bytes (%.2f MB)", rule.Status(), rule.Message(), allocDelta, float64(allocDelta)/(1<<20))
}

// TestLegacyPathFindAllConstPatternIsGuarded pins that findAll's guard survives the
// constant-pattern regex optimization k8s.io/apiserver's library.Regex installs. That
// optimization rebuilds the call node around its own closure, which discarded the guard
// decorator's wrapper: with a constant pattern a 1.5 MB target allocated ~154 MB and was
// refused only afterwards by the cost limit, while a variable pattern was refused by the
// guard at ~0 MB. Driven through validateResource, the same real path the test above uses.
func TestLegacyPathFindAllConstPatternIsGuarded(t *testing.T) {
	target := strings.Repeat("a", 1_500_000)

	rawResource, err := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":        "test-pod",
			"namespace":   "default",
			"annotations": map[string]string{"target": target, "pattern": "a"},
		},
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "nginx", "image": "nginx:latest"}},
		},
	})
	assert.NilError(t, err)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	restMapper, err := utils.GetRESTMapper(nil)
	assert.NilError(t, err)
	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	assert.NilError(t, err)
	gvr := mapping.Resource
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: resource.GetNamespace()}}

	// The constant form is the one the optimizer rewrites; the variable form never was, so it
	// is the control that shows the guard itself still works and nothing else changed.
	for _, tc := range []struct {
		name string
		expr string
	}{
		{"constant pattern", `size(object.metadata.annotations["target"].findAll("a")) >= 0`},
		{"variable pattern", `size(object.metadata.annotations["target"].findAll(object.metadata.annotations["pattern"])) >= 0`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vap-findall"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					Validations: []admissionregistrationv1.Validation{{Expression: tc.expr}},
				},
			}
			a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)

			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			response, err := validateResource(policy, nil, *resource, nil, ns, a)
			assert.NilError(t, err)

			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			allocDelta := after.TotalAlloc - before.TotalAlloc

			if len(response.PolicyResponse.Rules) != 1 {
				t.Fatalf("expected exactly one rule response, got %d", len(response.PolicyResponse.Rules))
			}
			rule := response.PolicyResponse.Rules[0]
			// The guard's own wording, not merely "refused": the cost limit also refuses this
			// shape, just after the allocation, so asserting refusal alone passes unguarded.
			if !strings.Contains(rule.Message(), "findAll target is") {
				t.Fatalf("expected the findAll target-cap guard refusal, got status=%q message=%q allocDelta=%d bytes (%.1f MB)",
					rule.Status(), rule.Message(), allocDelta, float64(allocDelta)/(1<<20))
			}
			if allocDelta > 32<<20 {
				t.Fatalf("allocation = %d bytes (%.1f MB), want < 32 MB -- the guard was bypassed and the refusal arrived after the work",
					allocDelta, float64(allocDelta)/(1<<20))
			}
			t.Logf("%s: status=%q message=%q allocDelta=%d bytes (%.2f MB)", tc.name, rule.Status(), rule.Message(), allocDelta, float64(allocDelta)/(1<<20))
		})
	}
}

// TestLegacyPathFindAllStillWorks is the correctness control for the change above: declining
// the apiserver's constant-pattern rewrite must not alter find/findAll semantics, including
// the optional limit argument.
func TestLegacyPathFindAllStillWorks(t *testing.T) {
	rawResource, err := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":        "test-pod",
			"namespace":   "default",
			"annotations": map[string]string{"s": "ab 12 cd 345"},
		},
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "nginx", "image": "nginx:latest"}},
		},
	})
	assert.NilError(t, err)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	restMapper, err := utils.GetRESTMapper(nil)
	assert.NilError(t, err)
	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	assert.NilError(t, err)
	gvr := mapping.Resource
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: resource.GetNamespace()}}

	for _, tc := range []struct {
		name string
		expr string
	}{
		{"findAll returns every match", `object.metadata.annotations["s"].findAll("[0-9]+") == ["12", "345"]`},
		{"findAll honours the limit argument", `object.metadata.annotations["s"].findAll("[0-9]+", 1) == ["12"]`},
		{"find returns the first match", `object.metadata.annotations["s"].find("[0-9]+") == "12"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "vap-findall-ok"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					Validations: []admissionregistrationv1.Validation{{Expression: tc.expr}},
				},
			}
			a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
			response, err := validateResource(policy, nil, *resource, nil, ns, a)
			assert.NilError(t, err)
			if len(response.PolicyResponse.Rules) != 1 {
				t.Fatalf("expected exactly one rule response, got %d", len(response.PolicyResponse.Rules))
			}
			if rule := response.PolicyResponse.Rules[0]; rule.Status() != "pass" {
				t.Fatalf("expected the expression to evaluate true (status pass), got status=%q message=%q", rule.Status(), rule.Message())
			}
		})
	}
}
