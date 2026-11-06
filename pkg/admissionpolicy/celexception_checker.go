package admissionpolicy

import (
	"fmt"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	"k8s.io/apiserver/pkg/cel/environment"
	"k8s.io/client-go/tools/cache"
)

// nativeMatchConditionCompiler compiles match conditions the way the API server does for a
// ValidatingAdmissionPolicy: base CEL environment and admission variables, no Kyverno libraries.
var nativeMatchConditionCompiler = cel.NewCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion()))

// CanGenerateNativePolicy reports whether a ValidatingPolicy still translates to a
// ValidatingAdmissionPolicy. Controls could be folded in as !(match && controls), but a VAP emits
// only the policy's own message, and it cannot admit a resource the policy already passes.
// Exception match conditions are copied into the VAP negated, so they must also compile without
// the Kyverno CEL libraries; otherwise the API server rejects the VAP and an existing VAP is left
// enforcing without the exception.
func CanGenerateNativePolicy(exceptions []policiesv1beta1.PolicyException) (bool, string) {
	var withControls, withKyvernoCEL []string
	for _, exception := range exceptions {
		name := cache.MetaObjectToName(&exception).String()
		if len(exception.Spec.Validations) > 0 {
			withControls = append(withControls, name)
		}
		if !nativeMatchConditionsCompile(exception) {
			withKyvernoCEL = append(withKyvernoCEL, name)
		}
	}
	var reasons []string
	if len(withControls) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
			strings.Join(withControls, ", "),
		))
	}
	if len(withKyvernoCEL) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"policy exception(s) %s use matchConditions that the API server cannot compile in a generated ValidatingAdmissionPolicy, such as calls to the Kyverno CEL libraries",
			strings.Join(withKyvernoCEL, ", "),
		))
	}
	if len(reasons) == 0 {
		return true, ""
	}
	return false, strings.Join(reasons, "; ")
}

// nativeMatchConditionsCompile reports whether the negated match conditions the VAP builder
// generates for the exception compile in the API server's admission environment.
func nativeMatchConditionsCompile(exception policiesv1beta1.PolicyException) bool {
	for _, condition := range negateExceptionMatchConditions([]policiesv1beta1.PolicyException{exception}) {
		result := nativeMatchConditionCompiler.CompileCELExpression(
			&matchconditions.MatchCondition{Name: condition.Name, Expression: condition.Expression},
			cel.OptionalVariableDeclarations{HasAuthorizer: true},
			environment.NewExpressions,
		)
		if result.Error != nil {
			return false
		}
	}
	return true
}
