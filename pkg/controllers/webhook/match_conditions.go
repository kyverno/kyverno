package webhook

import (
	"sync"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// namespaceObjectVarName is declared by the API server CEL environment used to
// compile webhook match conditions, but it is only populated for
// ValidatingAdmissionPolicy/MutatingAdmissionPolicy. For webhook match
// conditions the API server always binds it to null (see
// k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions), so any field
// access on it fails at evaluation time (e.g. "no such key: metadata").
const namespaceObjectVarName = "namespaceObject"

// webhookMatchConditions filters out match conditions that cannot be offloaded
// to the API server because they reference variables the API server never
// populates for webhook match conditions (namespaceObject). Offloaded match
// conditions are only a pre-filter: the Kyverno engine always re-evaluates the
// full set of match conditions with the resolved namespace, so dropping a
// condition here only means the corresponding requests are forwarded to Kyverno
// instead of being filtered by the API server.
func webhookMatchConditions(conditions []admissionregistrationv1.MatchCondition) []admissionregistrationv1.MatchCondition {
	var result []admissionregistrationv1.MatchCondition
	for _, condition := range conditions {
		if referencesNamespaceObject(condition.Expression) {
			continue
		}
		result = append(result, condition)
	}
	return result
}

// matchConditionParseEnv is a parse-only CEL environment: expressions are not
// type-checked, so no variable declarations are needed, but parsing must
// support the syntax accepted for match conditions (e.g. optional field
// selection).
var (
	matchConditionParseEnv     *cel.Env
	errMatchConditionParseEnv  error
	matchConditionParseEnvOnce sync.Once
)

func getMatchConditionParseEnv() (*cel.Env, error) {
	matchConditionParseEnvOnce.Do(func() {
		matchConditionParseEnv, errMatchConditionParseEnv = cel.NewEnv(cel.OptionalTypes())
	})
	return matchConditionParseEnv, errMatchConditionParseEnv
}

// referencesNamespaceObject reports whether the given CEL expression references
// the namespaceObject variable. If the expression cannot be parsed, it is
// conservatively treated as referencing it so evaluation stays on the Kyverno
// side.
func referencesNamespaceObject(expression string) bool {
	env, err := getMatchConditionParseEnv()
	if err != nil {
		return true
	}
	parsed, iss := env.Parse(expression)
	if iss != nil && iss.Err() != nil {
		return true
	}
	found := false
	celast.PreOrderVisit(parsed.NativeRep().Expr(), celast.NewExprVisitor(func(node celast.Expr) {
		if found || node == nil {
			return
		}
		if node.Kind() == celast.IdentKind && node.AsIdent() == namespaceObjectVarName {
			found = true
		}
	}))
	return found
}
