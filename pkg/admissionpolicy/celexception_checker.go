package admissionpolicy

import (
	"fmt"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/client-go/tools/cache"
)

// CanGenerateNativePolicy reports whether a ValidatingPolicy still translates to a
// ValidatingAdmissionPolicy. Controls could be folded in as !(match && controls), but a VAP emits
// only the policy's own message, and it cannot admit a resource the policy already passes.
func CanGenerateNativePolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var offending []string
	for _, exception := range exceptions {
		if len(exception.Spec.Validations) > 0 {
			offending = append(offending, cache.MetaObjectToName(&exception).String())
		}
	}
	if len(offending) == 0 {
		return true, ""
	}
	return false, fmt.Sprintf(
		"policy exception(s) %s define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
		strings.Join(offending, ", "),
	)
}
