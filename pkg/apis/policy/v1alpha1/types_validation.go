package v1alpha1

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Checks if rule is not empty and all substructures are valid
func (pr *PolicyRule) Validate() error {
	err := pr.Resource.Validate()
	if err != nil {
		return err
	}

	if len(pr.Patches) == 0 && pr.ConfigMapGenerator == nil && pr.SecretGenerator == nil {
		return errors.New("The rule is empty")
	}

	if len(pr.Patches) > 0 {
		for _, patch := range pr.Patches {
			err = patch.Validate()
			if err != nil {
				return err
			}
		}
	}

	if pr.ConfigMapGenerator != nil {
		err = pr.ConfigMapGenerator.Validate()
		if err != nil {
			return err
		}
	}

	if pr.SecretGenerator != nil {
		err = pr.SecretGenerator.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// Checks if all necesarry fields are present and have values. Also checks a Selector.
// Returns error if resource definition is invalid.
func (pr *PolicyResource) Validate() error {
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

// Checks if all mandatory PolicyPatch fields are set
func (pp *PolicyPatch) Validate() error {
	if pp.Path == "" {
		return errors.New("JSONPatch field 'path' is mandatory")
	}

	if pp.Operation == "add" || pp.Operation == "replace" {
		if pp.Value == nil {
			return errors.New(fmt.Sprintf("JSONPatch field 'value' is mandatory for operation '%s'", pp.Operation))
		}

		return nil
	} else if pp.Operation == "remove" {
		return nil
	}

	return errors.New(fmt.Sprintf("Unsupported JSONPatch operation '%s'", pp.Operation))
}

// Returns error if Name or namespace is not cpecified
func (pcf *PolicyCopyFrom) Validate() error {
	if pcf.Name == "" || pcf.Namespace == "" {
		return errors.New("Name or/and Namespace is not specified")
	}
	return nil
}

// Returns error if generator is configured incompletely
func (pcg *PolicyConfigGenerator) Validate() error {
	if pcg.Name == "" {
		return errors.New("The generator is unnamed")
	} else if len(pcg.Data) == 0 && pcg.CopyFrom == nil {
		return errors.New("Neither Data nor CopyFrom (source) is specified")
	}
	if pcg.CopyFrom != nil {
		return pcg.CopyFrom.Validate()
	}
	return nil
}
