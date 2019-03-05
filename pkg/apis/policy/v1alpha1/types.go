package v1alpha1

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// An example of the YAML representation of this structure is here:
// <project_root>/crd/policy-example.yaml
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PolicySpec   `json:"spec"`
	Status            PolicyStatus `json:"status"`
}

// Specification of the Policy.
type PolicySpec struct {
	FailurePolicy *string      `json:"failurePolicy"`
	Rules         []PolicyRule `json:"rules"`
}

// The rule of mutation for the single resource definition.
// Details are listed in the description of each of the substructures.
type PolicyRule struct {
	Resource           PolicyResource         `json:"resource"`
	Patches            []PolicyPatch          `json:"patch,omitempty"`
	ConfigMapGenerator *PolicyConfigGenerator `json:"configMapGenerator,omitempty"`
	SecretGenerator    *PolicyConfigGenerator `json:"secretGenerator,omitempty"`
}

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

// Describes the resource to which the PolicyRule will apply.
// Either the name or selector must be specified.
// IMPORTANT: If neither is specified, the policy rule will not apply (TBD).
type PolicyResource struct {
	Kind     string                `json:"kind"`
	Name     *string               `json:"name"`
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
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

// PolicyPatch declares patch operation for created object according to the JSONPatch spec:
// http://jsonpatch.com/
type PolicyPatch struct {
	Path      string `json:"path"`
	Operation string `json:"op"`
	Value     string `json:"value"`
}

func (pp *PolicyPatch) Validate() error {
	if pp.Path == "" {
		return errors.New("JSONPatch field 'path' is mandatory")
	}

	if pp.Operation == "add" || pp.Operation == "replace" {
		if pp.Value == "" {
			return errors.New(fmt.Sprintf("JSONPatch field 'value' is mandatory for operation '%s'", pp.Operation))
		}
		return nil
	} else if pp.Operation == "remove" {
		return nil
	}

	return errors.New(fmt.Sprintf("Unsupported JSONPatch operation '%s'", pp.Operation))
}

// The declaration for a Secret or a ConfigMap, which will be created in the new namespace.
// Can be applied only when PolicyRule.Resource.Kind is "Namespace".
type PolicyConfigGenerator struct {
	Name     string            `json:"name"`
	CopyFrom *PolicyCopyFrom   `json:"copyFrom"`
	Data     map[string]string `json:"data"`
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

// Location of a Secret or a ConfigMap which will be used as source when applying PolicyConfigGenerator
type PolicyCopyFrom struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// Returns error if Name or namespace is not cpecified
func (pcf *PolicyCopyFrom) Validate() error {
	if pcf.Name == "" || pcf.Namespace == "" {
		return errors.New("Name or/and Namespace is not specified")
	}
	return nil
}

// Contains logs about policy application
type PolicyStatus struct {
	Logs []string `json:"log"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// List of Policy resources
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Policy `json:"items"`
}
