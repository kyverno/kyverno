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
func (gen *Generation) Validate() error {
	if gen.Data == nil && gen.Clone == (CloneFrom{}) {
		return fmt.Errorf("Neither data nor clone (source) of %s is specified", gen.Kind)
	}
	if gen.Data != nil && gen.Clone != (CloneFrom{}) {
		return fmt.Errorf("Both data nor clone (source) of %s are specified", gen.Kind)
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
func (gen *Generation) DeepCopyInto(out *Generation) {
	if out != nil {
		*out = *gen
	}
}

//ToKey generates the key string used for adding label to polivy violation
func (rs ResourceSpec) ToKey() string {
	if rs.Namespace == "" {
		return rs.Kind + "." + rs.Name
	}
	return rs.Kind + "." + rs.Namespace + "." + rs.Name
}
