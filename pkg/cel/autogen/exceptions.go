package autogen

import (
	"slices"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

// RewriteExceptions rewrites PolicyException match conditions and validations using the same
// spec field-path replacements applied to the policy spec for the given autogen
// config, so exceptions match against the autogen'd controller's shape
// instead of the original Pod-shaped policy. Controller metadata paths are preserved:
// metadata replacements are not applied, so expressions such as `object.metadata.name`
// keep referring to the controller itself rather than its pod template.
func RewriteExceptions(exceptions []*policiesv1beta1.PolicyException, config string) ([]*policiesv1beta1.PolicyException, error) {
	if len(exceptions) == 0 {
		return nil, nil
	}
	replacements := slices.DeleteFunc(slices.Clone(ReplacementsMap[config]), func(r Replacement) bool {
		return r.From == "metadata"
	})
	if len(replacements) == 0 {
		return slices.Clone(exceptions), nil
	}

	out := make([]*policiesv1beta1.PolicyException, 0, len(exceptions))
	for _, polex := range exceptions {
		polex := polex.DeepCopy()

		for i := range polex.Spec.MatchConditions {
			expr := polex.Spec.MatchConditions[i].Expression
			if expr != "" {
				polex.Spec.MatchConditions[i].Expression = string(ApplyCEL([]byte(expr), replacements...))
			}
		}

		for i := range polex.Spec.Validations {
			expr := polex.Spec.Validations[i].Expression
			if expr != "" {
				polex.Spec.Validations[i].Expression = string(ApplyCEL([]byte(expr), replacements...))
			}
			msgExpr := polex.Spec.Validations[i].MessageExpression
			if msgExpr != "" {
				polex.Spec.Validations[i].MessageExpression = string(ApplyCEL([]byte(msgExpr), replacements...))
			}
		}

		out = append(out, polex)
	}
	return out, nil
}
