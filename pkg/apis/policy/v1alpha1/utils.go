package v1alpha1

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate checks if rule is not empty and all substructures are valid
func (r *Rule) Validate() error {
	// check matches Resoource Description of match resource
	err := r.MatchResources.ResourceDescription.Validate()
	if err != nil {
		return err
	}

	if r.Mutation == nil && r.Validation == nil && r.Generation == nil {
		return errors.New("The rule is empty")
	}

	return nil
}

// Validate checks if all necesarry fields are present and have values. Also checks a Selector.
// Returns error if
// - kinds is not defined
func (pr *ResourceDescription) Validate() error {
	if len(pr.Kinds) == 0 {
		return errors.New("The Kind is not specified")
	}

	if pr.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(pr.Selector)
		if err != nil {
			return err
		}
		requirements, _ := selector.Requirements()
		if len(requirements) == 0 {
			return errors.New("The requirements are not specified in selector")
		}
	}

	return nil
}

// Validate if all mandatory PolicyPatch fields are set
func (pp *Patch) Validate() error {
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

	return fmt.Errorf("Unsupported JSONPatch operation '%s'", pp.Operation)
}

// Validate returns error if generator is configured incompletely
func (pcg *Generation) Validate() error {
	if pcg.Data == nil && pcg.Clone == nil {
		return fmt.Errorf("Neither data nor clone (source) of %s is specified", pcg.Kind)
	}
	if pcg.Data != nil && pcg.Clone != nil {
		return fmt.Errorf("Both data nor clone (source) of %s are specified", pcg.Kind)
	}
	return nil
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (in *Mutation) DeepCopyInto(out *Mutation) {
	if out != nil {
		*out = *in
	}
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (pp *Patch) DeepCopyInto(out *Patch) {
	if out != nil {
		*out = *pp
	}
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (in *Validation) DeepCopyInto(out *Validation) {
	if out != nil {
		*out = *in
	}
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (in *Generation) DeepCopyInto(out *Generation) {
	if out != nil {
		*out = *in
	}
}

// return true -> if there were any removals
// return false -> if it looks the same
func (v *Violation) RemoveRulesOfType(ruleType string) bool {
	removed := false
	updatedRules := []FailedRule{}
	for _, r := range v.Rules {
		if r.Type == ruleType {
			removed = true
			continue
		}
		updatedRules = append(updatedRules, r)
	}

	if removed {
		v.Rules = updatedRules
		return true
	}
	return false
}

//IsEqual Check if violatiosn are equal
func (v *Violation) IsEqual(nv Violation) bool {
	// We do not need to compare resource info as it will be same
	// Reason
	if v.Reason != nv.Reason {
		return false
	}
	// Rule
	if len(v.Rules) != len(nv.Rules) {
		return false
	}
	// assumes the rules will be in order, as the rule are proceeed in order
	// if the rule order changes, it means the policy has changed.. as it will afffect the order in which mutation rules are applied
	for i, r := range v.Rules {
		if r != nv.Rules[i] {
			return false
		}
	}
	return true
}
