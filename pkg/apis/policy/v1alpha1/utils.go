package v1alpha1

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate checks if rule is not empty and all substructures are valid
func (r *Rule) Validate() error {
	err := r.ResourceDescription.Validate()
	if err != nil {
		return err
	}

	if r.Mutation == nil && r.Validation == nil && r.Generation == nil {
		return errors.New("The rule is empty")
	}

	return nil
}

// Validate checks if all necesarry fields are present and have values. Also checks a Selector.
// Returns error if resource definition is invalid.
func (pr *ResourceDescription) Validate() error {
	// TBD: selector or name MUST be specified
	if pr.Kind == "" {
		return errors.New("The Kind is not specified")
	} else if pr.Name == nil && pr.Selector == nil {
		return errors.New("Neither Name nor Selector is specified")
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

// Validate returns error if Name or namespace is not cpecified
func (pcf *CopyFrom) Validate() error {
	if pcf.Name == "" || pcf.Namespace == "" {
		return errors.New("Name or/and Namespace is not specified")
	}
	return nil
}

// Validate returns error if generator is configured incompletely
func (pcg *Generation) Validate() error {
	if pcg.Name == "" || pcg.Kind == "" {
		return errors.New("Name or/and Kind of generator is not specified")
	}

	if pcg.CopyFrom != nil {
		return pcg.CopyFrom.Validate()
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
