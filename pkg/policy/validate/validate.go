package validate

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"github.com/kyverno/kyverno/pkg/policy/common"
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
		if path, err := common.ValidatePattern(rule.Pattern, "/", []commonAnchors.IsAnchor{commonAnchors.IsConditionAnchor, commonAnchors.IsExistenceAnchor, commonAnchors.IsEqualityAnchor, commonAnchors.IsNegationAnchor}); err != nil {
			return fmt.Sprintf("pattern.%s", path), err
		}
	}

	if rule.AnyPattern != nil {
		anyPattern, err := rule.DeserializeAnyPattern()
		if err != nil {
			return "anyPattern", fmt.Errorf("failed to deserialize anyPattern, expect array: %v", err)
		}
		for i, pattern := range anyPattern {
			if path, err := common.ValidatePattern(pattern, "/", []commonAnchors.IsAnchor{commonAnchors.IsConditionAnchor, commonAnchors.IsExistenceAnchor, commonAnchors.IsEqualityAnchor, commonAnchors.IsNegationAnchor}); err != nil {
				return fmt.Sprintf("anyPattern[%d].%s", i, path), err
			}
		}
	}
	return "", nil
}

// validateOverlayPattern checks one of pattern/anyPattern must exist
func (v *Validate) validateOverlayPattern() error {
	rule := v.rule
	if rule.Pattern == nil && rule.AnyPattern == nil && rule.Deny == nil {
		return fmt.Errorf("pattern, anyPattern or deny must be specified")
	}

	if rule.Pattern != nil && rule.AnyPattern != nil {
		return fmt.Errorf("only one operation allowed per validation rule(pattern or anyPattern)")
	}

	return nil
}
