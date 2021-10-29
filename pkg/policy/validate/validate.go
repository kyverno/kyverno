package validate

import (
	"fmt"
	"strings"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"github.com/kyverno/kyverno/pkg/policy/common"
)

// Validate validates a 'validate' rule
type Validate struct {
	// rule to hold 'validate' rule specifications
	rule *kyverno.Validation
}

//NewValidateFactory returns a new instance of Mutate validation checker
func NewValidateFactory(rule *kyverno.Validation) *Validate {
	m := Validate{
		rule: rule,
	}

	return &m
}

//Validate validates the 'validate' rule
func (v *Validate) Validate() (string, error) {
	if err := v.validateElements(); err != nil {
		// no need to proceed ahead
		return "", err
	}

	if v.rule.Pattern != nil {
		if path, err := common.ValidatePattern(v.rule.Pattern, "/", []commonAnchors.IsAnchor{commonAnchors.IsConditionAnchor, commonAnchors.IsExistenceAnchor, commonAnchors.IsEqualityAnchor, commonAnchors.IsNegationAnchor, commonAnchors.IsGlobalAnchor}); err != nil {
			return fmt.Sprintf("pattern.%s", path), err
		}
	}

	if v.rule.AnyPattern != nil {
		anyPattern, err := v.rule.DeserializeAnyPattern()
		if err != nil {
			return "anyPattern", fmt.Errorf("failed to deserialize anyPattern, expect array: %v", err)
		}
		for i, pattern := range anyPattern {
			if path, err := common.ValidatePattern(pattern, "/", []commonAnchors.IsAnchor{commonAnchors.IsConditionAnchor, commonAnchors.IsExistenceAnchor, commonAnchors.IsEqualityAnchor, commonAnchors.IsNegationAnchor, commonAnchors.IsGlobalAnchor}); err != nil {
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

func validationElemCount(v *kyverno.Validation) int {
	if v == nil {
		return 0
	}

	count := 0
	if v.Pattern != nil {
		count++
	}

	if v.AnyPattern != nil {
		count++
	}

	if v.Deny != nil {
		count++
	}

	if v.ForEachValidation != nil {
		count++
	}

	return count
}

func (v *Validate) validateForEach(foreach *kyverno.ForEachValidation) error {
	if foreach.List == "" {
		return fmt.Errorf("foreach.list is required")
	}

	if !strings.HasPrefix(foreach.List, "request.object") {
		return fmt.Errorf("foreach.list must start with 'request.object' e.g. 'request.object.spec.containers'")
	}

	count := foreachElemCount(foreach)
	if count == 0 {
		return fmt.Errorf("one of pattern, anyPattern, deny must be specified")
	}

	if count > 1 {
		return fmt.Errorf("only one of pattern, anyPattern, deny can be specified")
	}

	return nil
}

func foreachElemCount(foreach *kyverno.ForEachValidation) int {
	if foreach == nil {
		return 0
	}

	count := 0
	if foreach.Pattern != nil {
		count++
	}

	if foreach.AnyPattern != nil {
		count++
	}

	if foreach.Deny != nil {
		count++
	}

	return count
}
