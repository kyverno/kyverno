package validate

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/policy/common"
)

// Validate validates a 'validate' rule
type Validate struct {
	// rule to hold 'validate' rule specifications
	rule *kyvernov1.Validation
}

// NewValidateFactory returns a new instance of Mutate validation checker
func NewValidateFactory(rule *kyvernov1.Validation) *Validate {
	m := Validate{
		rule: rule,
	}

	return &m
}

// Validate validates the 'validate' rule
func (v *Validate) Validate(ctx context.Context) (string, error) {
	if err := v.validateElements(); err != nil {
		return "", err
	}

	if target := v.rule.GetPattern(); target != nil {
		if path, err := common.ValidatePattern(target, "/", func(a anchor.Anchor) bool {
			return anchor.IsCondition(a) ||
				anchor.IsExistence(a) ||
				anchor.IsEquality(a) ||
				anchor.IsNegation(a) ||
				anchor.IsGlobal(a)
		}); err != nil {
			return fmt.Sprintf("pattern.%s", path), err
		}
	}

	if target := v.rule.GetAnyPattern(); target != nil {
		anyPattern, err := v.rule.DeserializeAnyPattern()
		if err != nil {
			return "anyPattern", fmt.Errorf("failed to deserialize anyPattern, expect array: %v", err)
		}
		for i, pattern := range anyPattern {
			if path, err := common.ValidatePattern(pattern, "/", func(a anchor.Anchor) bool {
				return anchor.IsCondition(a) ||
					anchor.IsExistence(a) ||
					anchor.IsEquality(a) ||
					anchor.IsNegation(a) ||
					anchor.IsGlobal(a)
			}); err != nil {
				return fmt.Sprintf("anyPattern[%d].%s", i, path), err
			}
		}
	}

	if v.rule.ForEachValidation != nil {
		for _, foreach := range v.rule.ForEachValidation {
			if err := v.validateForEach(foreach); err != nil {
				return "", err
			}
		}
	}

	return "", nil
}

func (v *Validate) validateElements() error {
	count := validationElemCount(v.rule)
	if count == 0 {
		return fmt.Errorf("one of pattern, anyPattern, deny, foreach must be specified")
	}

	if count > 1 {
		return fmt.Errorf("only one of pattern, anyPattern, deny, foreach can be specified")
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

	if v.Manifests != nil && len(v.Manifests.Attestors) != 0 {
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

	if foreach.ForEachValidation != nil {
		count++
	}

	return count
}
