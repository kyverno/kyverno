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
