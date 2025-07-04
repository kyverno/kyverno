package validate

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/auth"
	"github.com/kyverno/kyverno/pkg/policy/auth/fake"
	"github.com/kyverno/kyverno/pkg/policy/common"
)

// Validate validates a 'validate' rule
type Validate struct {
	rule           *kyvernov1.Rule
	validationRule *kyvernov1.Validation
	authChecker    auth.AuthChecks
}

// NewValidateFactory returns a new instance of Mutate validation checker
func NewValidateFactory(rule *kyvernov1.Rule, client dclient.Interface, mock bool, reportsSA string) *Validate {
	var authChecker auth.AuthChecks
	if mock {
		authChecker = fake.NewFakeAuth()
	} else {
		authChecker = auth.NewAuth(client, reportsSA, logging.GlobalLogger())
	}

	return &Validate{
		rule:           rule,
		validationRule: rule.Validation,
		authChecker:    authChecker,
	}
}

func NewMockValidateFactory(rule *kyvernov1.Rule) *Validate {
	return &Validate{
		rule:           rule,
		validationRule: rule.Validation,
		authChecker:    fake.NewFakeAuth(),
	}
}

// Validate validates the 'validate' rule
func (v *Validate) Validate(ctx context.Context, _ []string) (warnings []string, path string, err error) {
	if err := v.validateElements(); err != nil {
		return nil, "", err
	}

	if target := v.validationRule.GetPattern(); target != nil {
		if path, err := common.ValidatePattern(target, "/", func(a anchor.Anchor) bool {
			return anchor.IsCondition(a) ||
				anchor.IsExistence(a) ||
				anchor.IsEquality(a) ||
				anchor.IsNegation(a) ||
				anchor.IsGlobal(a)
		}); err != nil {
			return nil, fmt.Sprintf("pattern.%s", path), err
		}
	}

	if target := v.validationRule.GetAnyPattern(); target != nil {
		anyPattern, err := v.validationRule.DeserializeAnyPattern()
		if err != nil {
			return nil, "anyPattern", fmt.Errorf("failed to deserialize anyPattern, expect array: %v", err)
		}
		for i, pattern := range anyPattern {
			if path, err := common.ValidatePattern(pattern, "/", func(a anchor.Anchor) bool {
				return anchor.IsCondition(a) ||
					anchor.IsExistence(a) ||
					anchor.IsEquality(a) ||
					anchor.IsNegation(a) ||
					anchor.IsGlobal(a)
			}); err != nil {
				return nil, fmt.Sprintf("anyPattern[%d].%s", i, path), err
			}
		}
	}

	if v.validationRule.ForEachValidation != nil {
		for _, foreach := range v.validationRule.ForEachValidation {
			if err := v.validateForEach(foreach); err != nil {
				return nil, "", err
			}
		}
	}
	if v.rule.CELPreconditions != nil && v.validationRule.CEL == nil {
		return nil, "", fmt.Errorf("celPrecondition can only be used with validate.cel")
	}

	if v.validationRule.CEL != nil {
		for _, expression := range v.validationRule.CEL.Expressions {
			if expression.Expression == "" {
				return nil, "", fmt.Errorf("cel.expressions.expression is required")
			}
		}

		if v.validationRule.CEL.ParamKind != nil {
			if v.validationRule.CEL.ParamKind.APIVersion == "" {
				return nil, "", fmt.Errorf("cel.paramKind.apiVersion is required")
			}

			if v.validationRule.CEL.ParamKind.Kind == "" {
				return nil, "", fmt.Errorf("cel.paramKind.kind is required")
			}

			if v.validationRule.CEL.ParamRef == nil {
				return nil, "", fmt.Errorf("cel.paramRef is required")
			}
		}

		if v.validationRule.CEL.ParamRef != nil {
			if v.validationRule.CEL.ParamRef.Name == "" && v.validationRule.CEL.ParamRef.Selector == nil {
				return nil, "", fmt.Errorf("one of cel.paramRef.name or cel.paramRef.selector must be set")
			}

			if v.validationRule.CEL.ParamRef.Name != "" && v.validationRule.CEL.ParamRef.Selector != nil {
				return nil, "", fmt.Errorf("one of cel.paramRef.name or cel.paramRef.selector must be set")
			}

			if v.validationRule.CEL.ParamRef.ParameterNotFoundAction == nil {
				return nil, "", fmt.Errorf("cel.paramRef.parameterNotFoundAction is required")
			}

			if v.validationRule.CEL.ParamKind == nil {
				return nil, "", fmt.Errorf("cel.paramKind is required")
			}
		}

		if v.validationRule.CEL.AuditAnnotations != nil {
			for _, auditAnnotation := range v.validationRule.CEL.AuditAnnotations {
				if auditAnnotation.Key == "" {
					return nil, "", fmt.Errorf("cel.auditAnnotation.key is required")
				}

				if auditAnnotation.ValueExpression == "" {
					return nil, "", fmt.Errorf("cel.auditAnnotation.valueExpression is required")
				}
			}
		}
	}

	if w, err := v.validateAuth(ctx); err != nil {
		return nil, "", err
	} else if len(w) > 0 {
		warnings = append(warnings, w...)
	}

	return warnings, "", nil
}

func (v *Validate) validateAuth(ctx context.Context) (warnings []string, err error) {
	kinds := v.rule.MatchResources.GetKinds()
	for _, k := range kinds {
		if wildcard.ContainsWildcard(k) {
			return nil, nil
		}

		verbs := []string{"get", "list", "watch"}
		ok, msg, err := v.authChecker.CanI(ctx, verbs, k, "", "", "")
		if err != nil {
			return nil, err
		}
		if !ok {
			return []string{msg}, nil
		}
	}

	return nil, nil
}

func (v *Validate) validateElements() error {
	count := validationElemCount(v.validationRule)
	if count == 0 {
		return fmt.Errorf("one of pattern, anyPattern, deny, foreach, cel must be specified")
	}

	if count > 1 {
		return fmt.Errorf("only one of pattern, anyPattern, deny, foreach, cel can be specified")
	}

	return nil
}

func validationElemCount(v *kyvernov1.Validation) int {
	if v == nil {
		return 0
	}

	count := 0
	if v.GetPattern() != nil {
		count++
	}

	if v.GetAnyPattern() != nil {
		count++
	}

	if v.Deny != nil {
		count++
	}

	if v.ForEachValidation != nil {
		count++
	}

	if v.PodSecurity != nil {
		count++
	}

	if v.CEL != nil {
		count++
	}

	if v.Manifests != nil && len(v.Manifests.Attestors) != 0 {
		count++
	}

	if v.Assert.Value != nil {
		count++
	}

	return count
}

func (v *Validate) validateForEach(foreach kyvernov1.ForEachValidation) error {
	if foreach.List == "" {
		return fmt.Errorf("foreach.list is required")
	}

	count := foreachElemCount(foreach)
	if count == 0 {
		return fmt.Errorf("one of pattern, anyPattern, deny, or a nested foreach must be specified")
	}

	if count > 1 {
		return fmt.Errorf("only one of pattern, anyPattern, deny, or a nested foreach can be specified")
	}

	return nil
}

func foreachElemCount(foreach kyvernov1.ForEachValidation) int {
	count := 0
	if foreach.GetPattern() != nil {
		count++
	}

	if foreach.GetAnyPattern() != nil {
		count++
	}

	if foreach.Deny != nil {
		count++
	}

	if foreach.GetForEachValidation() != nil && len(foreach.GetForEachValidation()) > 0 {
		count++
	}

	return count
}
