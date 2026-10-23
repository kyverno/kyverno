package v1

import (
	"github.com/kyverno/kyverno/api/kyverno"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Aliases for legacy consumers; the types live in api/kyverno. Retained (regenerated) CRD types
// must reference kyverno.* directly, never these aliases: controller-gen takes the description of a
// non-inline field from the type name it sees, so a generated type that named an alias would pick
// up this comment. Removed with api/kyverno/v1 in #17713 section 3.

type (
	ResourceSpec = kyverno.ResourceSpec
)

// TargetResourceSpec defines targets for mutating existing resources.
type TargetResourceSpec struct {
	// TargetSelector contains the ResourceSpec and a label selector to support selecting with labels.
	TargetSelector `json:",omitempty"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []ContextEntry `json:"context,omitempty"`

	// Preconditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements. A direct list
	// of conditions (without `any` or `all` statements is supported for backwards compatibility but
	// will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	RawAnyAllConditions *ConditionsWrapper `json:"preconditions,omitempty"`
}

type TargetSelector struct {
	// ResourceSpec contains the target resources to load when mutating existing resources.
	ResourceSpec `json:",omitempty"`
	// Selector allows you to select target resources with their labels.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

func (r *TargetResourceSpec) GetAnyAllConditions() any {
	if r.RawAnyAllConditions == nil {
		return nil
	}
	return r.RawAnyAllConditions.Conditions
}

func (r *TargetResourceSpec) GetSelector() *metav1.LabelSelector { return r.Selector }
