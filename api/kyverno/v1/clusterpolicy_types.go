package v1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ClusterPolicy declares validation, mutation, and generation behaviors for matching resources.
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterpolicies,scope="Cluster",shortName=cpol
// +kubebuilder:printcolumn:name="Background",type="string",JSONPath=".spec.background"
// +kubebuilder:printcolumn:name="Action",type="string",JSONPath=".spec.validationFailureAction"
// +kubebuilder:printcolumn:name="Failure Policy",type="string",JSONPath=".spec.failurePolicy",priority=1
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`
type ClusterPolicy struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec declares policy behaviors.
	Spec Spec `json:"spec" yaml:"spec"`

	// Status contains policy runtime data.
	// +optional
	Status PolicyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// GetRules returns the policy rules
func (p *ClusterPolicy) GetRules() []Rule {
	return p.Spec.GetRules()
}

// HasAutoGenAnnotation checks if a policy has auto-gen annotation
func (p *ClusterPolicy) HasAutoGenAnnotation() bool {
	annotations := p.GetAnnotations()
	val, ok := annotations[PodControllersAnnotation]
	if ok && strings.ToLower(val) != "none" {
		return true
	}
	return false
}

// HasMutateOrValidateOrGenerate checks for rule types
func (p *ClusterPolicy) HasMutateOrValidateOrGenerate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() || rule.HasValidate() || rule.HasGenerate() {
			return true
		}
	}
	return false
}

// HasMutate checks for mutate rule types
func (p *ClusterPolicy) HasMutate() bool {
	return p.Spec.HasMutate()
}

// HasValidate checks for validate rule types
func (p *ClusterPolicy) HasValidate() bool {
	return p.Spec.HasValidate()
}

// HasGenerate checks for generate rule types
func (p *ClusterPolicy) HasGenerate() bool {
	return p.Spec.HasGenerate()
}

// HasVerifyImages checks for image verification rule types
func (p *ClusterPolicy) HasVerifyImages() bool {
	return p.Spec.HasVerifyImages()
}

// BackgroundProcessingEnabled checks if background is set to true
func (p *ClusterPolicy) BackgroundProcessingEnabled() bool {
	return p.Spec.BackgroundProcessingEnabled()
}

// Validate implements programmatic validation
func (p *ClusterPolicy) Validate() field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, ValidatePolicyName(field.NewPath("name"), p.Name)...)
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

// ClusterPolicyList is a list of ClusterPolicy instances.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterPolicyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []ClusterPolicy `json:"items" yaml:"items"`
}
