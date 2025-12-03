package v1beta1

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type (
	PolicyExceptionSpec = v1alpha1.PolicyExceptionSpec
	PolicyRef           = v1alpha1.PolicyRef
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

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyException `json:"items"`
}
