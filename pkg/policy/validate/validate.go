package validate

import (
	"fmt"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
	"github.com/nirmata/kyverno/pkg/policy/common"
)

// Validate provides implementation to validate 'validate' rule
type Validate struct {
	// rule to hold 'validate' rule specifications
	rule kyverno.Validation
}

//NewValidateFactory returns a new instance of Mutate validation checker
func NewValidateFactory(rule kyverno.Validation) *Validate {
	m := Validate{
		rule: rule,
	}
	return &m
}

//Validate validates the 'validate' rule
func (v *Validate) Validate() (string, error) {
	rule := v.rule
	if err := v.validateOverlayPattern(); err != nil {
		// no need to proceed ahead
		return "", err
	}

	if rule.Pattern != nil {
		if path, err := common.ValidatePattern(rule.Pattern, "/", []anchor.IsAnchor{anchor.IsConditionAnchor, anchor.IsExistenceAnchor, anchor.IsEqualityAnchor, anchor.IsNegationAnchor}); err != nil {
			return fmt.Sprintf("pattern.%s", path), err
		}
	}

	if len(rule.AnyPattern) != 0 {
		for i, pattern := range rule.AnyPattern {
			if path, err := common.ValidatePattern(pattern, "/", []anchor.IsAnchor{anchor.IsConditionAnchor, anchor.IsExistenceAnchor, anchor.IsEqualityAnchor, anchor.IsNegationAnchor}); err != nil {
				return fmt.Sprintf("anyPattern[%d].%s", i, path), err
			}
		}
	}
	return "", nil
}

// validateOverlayPattern checks one of pattern/anyPattern must exist
func (v *Validate) validateOverlayPattern() error {
	rule := v.rule
	if rule.Pattern == nil && len(rule.AnyPattern) == 0 && rule.Deny == nil {
		return fmt.Errorf("pattern, anyPattern or deny must be specified")
	}

	if rule.Pattern != nil && len(rule.AnyPattern) != 0 {
		return fmt.Errorf("only one operation allowed per validation rule(pattern or anyPattern)")
	}

	return nil
}
