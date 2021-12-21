package mutate

import (
	"errors"
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"github.com/kyverno/kyverno/pkg/policy/common"
)

// Mutate provides implementation to validate 'mutate' rule
type Mutate struct {
	// rule to hold 'mutate' rule specifications
	rule kyverno.Mutation
}

//NewMutateFactory returns a new instance of Mutate validation checker
func NewMutateFactory(rule kyverno.Mutation) *Mutate {
	m := Mutate{
		rule: rule,
	}
	return &m
}

//Validate validates the 'mutate' rule
func (m *Mutate) Validate() (string, error) {
	rule := m.rule
	// JSON Patches
	if len(rule.Patches) != 0 {
		for i, patch := range rule.Patches {
			if err := validatePatch(patch); err != nil {
				return fmt.Sprintf("patch[%d]", i), err
			}
		}
	}
	// Overlay
	if rule.Overlay != nil {
		path, err := common.ValidatePattern(rule.Overlay, "/", []commonAnchors.IsAnchor{commonAnchors.IsConditionAnchor, commonAnchors.IsAddingAnchor})
		if err != nil {
			return path, err
		}
	}
	return "", nil
}

// Validate if all mandatory PolicyPatch fields are set
func validatePatch(pp kyverno.Patch) error {
	if pp.Path == "" {
		return errors.New("JSONPatch field 'path' is mandatory")
	}
	if pp.Operation == "add" || pp.Operation == "replace" {
		if pp.Value == nil {
			return fmt.Errorf("JSONPatch field 'value' is mandatory for operation '%s'", pp.Operation)
		}

		return nil
	} else if pp.Operation == "remove" {
		return nil
	}

	return fmt.Errorf("unsupported JSONPatch operation '%s'", pp.Operation)
}
