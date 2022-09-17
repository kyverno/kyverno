package v1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Background",type=boolean,JSONPath=".spec.background"
// +kubebuilder:printcolumn:name="Validate Action",type=string,JSONPath=".spec.validationFailureAction"
// +kubebuilder:printcolumn:name="Failure Policy",type=string,JSONPath=".spec.failurePolicy",priority=1
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:resource:shortName=pol
// +kubebuilder:storageversion

// Policy declares validation, mutation, and generation behaviors for matching resources.
// See: https://kyverno.io/docs/writing-policies/ for more information.
type Policy struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec defines policy behaviors and contains one or more rules.
	Spec Spec `json:"spec" yaml:"spec"`

	// Status contains policy runtime information.
	// +optional
	// Deprecated. Policy metrics are available via the metrics endpoint
	Status PolicyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// HasAutoGenAnnotation checks if a policy has auto-gen annotation
func (p *Policy) HasAutoGenAnnotation() bool {
	annotations := p.GetAnnotations()
	val, ok := annotations[PodControllersAnnotation]
	if ok && strings.ToLower(val) != "none" {
		return true
	}
	return false
}

// HasMutateOrValidateOrGenerate checks for rule types
func (p *Policy) HasMutateOrValidateOrGenerate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() || rule.HasValidate() || rule.HasGenerate() {
			return true
		}
	}
	return false
}

// HasMutate checks for mutate rule types
func (p *Policy) HasMutate() bool {
	return p.Spec.HasMutate()
}

// HasValidate checks for validate rule types
func (p *Policy) HasValidate() bool {
	return p.Spec.HasValidate()
}

// HasGenerate checks for generate rule types
func (p *Policy) HasGenerate() bool {
	return p.Spec.HasGenerate()
}

// HasVerifyImages checks for image verification rule types
func (p *Policy) HasVerifyImages() bool {
	return p.Spec.HasVerifyImages()
}

// BackgroundProcessingEnabled checks if background is set to true
func (p *Policy) BackgroundProcessingEnabled() bool {
	return p.Spec.BackgroundProcessingEnabled()
}

// GetSpec returns the policy spec
func (p *Policy) GetSpec() *Spec {
	return &p.Spec
}

// IsNamespaced indicates if the policy is namespace scoped
func (p *Policy) IsNamespaced() bool {
	return true
}

// IsReady indicates if the policy is ready to serve the admission request
func (p *Policy) IsReady() bool {
	return p.Status.IsReady()
}

// Validate implements programmatic validation.
// namespaced means that the policy is bound to a namespace and therefore
// should not filter/generate cluster wide resources.
func (p *Policy) Validate(clusterResources sets.String) (errs field.ErrorList) {
	errs = append(errs, ValidateAutogenAnnotation(field.NewPath("metadata").Child("annotations"), p.GetAnnotations())...)
	errs = append(errs, ValidatePolicyName(field.NewPath("name"), p.Name)...)
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"), p.IsNamespaced(), clusterResources)...)
	return errs
}

func (p *Policy) GetKind() string {
	return p.Kind
}

func (p *Policy) CreateDeepCopy() PolicyInterface {
	return p.DeepCopy()
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyList is a list of Policy instances.
type PolicyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []Policy `json:"items" yaml:"items"`
}
