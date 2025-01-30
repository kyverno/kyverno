package v2alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyException declares resources to be excluded from specified policies.
type CELPolicyException struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec CELPolicyExceptionSpec `json:"spec"`
}

func (p *CELPolicyException) GetKind() string {
	return "CELPolicyException"
}

// Validate implements programmatic validation
func (p *CELPolicyException) Validate() (errs field.ErrorList) {
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

// PolicyExceptionSpec stores policy exception spec
type CELPolicyExceptionSpec struct {
	// PolicyNames identifies the policies to which the exception is applied.
	PolicyNames []string `json:"policyNames"`

	// MatchConstraints is used to check if a resource applies to the exception.
	MatchConstraints *admissionregistrationv1.MatchResources `json:"matchConstraints"`

	// MatchConditions is a list of CEL expressions that must be met for a resource to be excluded.
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty"`
}

// Validate implements programmatic validation
func (p *CELPolicyExceptionSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if len(p.PolicyNames) == 0 {
		errs = append(errs, field.Invalid(path.Child("policyNames"), p.PolicyNames, "must specify at least one policy name"))
	}
	if p.MatchConstraints == nil {
		errs = append(errs, field.Invalid(path.Child("matchConstraints"), p.MatchConstraints, "must specify match constraints"))
	}
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CELPolicyExceptionList is a list of Policy Exceptions
type CELPolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CELPolicyException `json:"items"`
}
