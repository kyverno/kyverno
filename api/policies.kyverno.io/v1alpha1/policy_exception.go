package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyException declares resources to be excluded from specified policies.
type PolicyException struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec PolicyExceptionSpec `json:"spec"`
}

func (p *PolicyException) GetKind() string {
	return "PolicyException"
}

// Validate implements programmatic validation
func (p *PolicyException) Validate() (errs field.ErrorList) {
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

// PolicyExceptionSpec stores policy exception spec
type PolicyExceptionSpec struct {
	// PolicyRefs identifies the policies to which the exception is applied.
	PolicyRefs []PolicyRef `json:"policyRefs"`

	// MatchConditions is a list of CEL expressions that must be met for a resource to be excluded.
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty"`
}

// Validate implements programmatic validation
func (p *PolicyExceptionSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if len(p.PolicyRefs) == 0 {
		errs = append(errs, field.Invalid(path.Child("policyRefs"), p.PolicyRefs, "must specify at least one policy ref"))
	} else {
		for i, policyRef := range p.PolicyRefs {
			errs = append(errs, policyRef.Validate(path.Child("policyRefs").Index(i))...)
		}
	}
	return errs
}

type PolicyRef struct {
	// Name is the name of the policy
	Name string `json:"name"`

	// Kind is the kind of the policy
	Kind string `json:"kind"`
}

func (p *PolicyRef) Validate(path *field.Path) (errs field.ErrorList) {
	if p.Name == "" {
		errs = append(errs, field.Invalid(path.Child("name"), p.Name, "must specify policy name"))
	}
	if p.Kind == "" {
		errs = append(errs, field.Invalid(path.Child("kind"), p.Kind, "must specify policy kind"))
	}
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyException `json:"items"`
}
