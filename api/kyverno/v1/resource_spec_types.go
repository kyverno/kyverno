package v1

import (
	"github.com/kyverno/kyverno/api/kyverno"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// TargetResourceSpec defines targets for mutating existing resources.
type TargetResourceSpec struct {
	// ResourceSpec contains the target resources to load when mutating existing resources.
	kyverno.ResourceSpec `json:",omitempty" yaml:",omitempty"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

	// Preconditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements. A direct list
	// of conditions (without `any` or `all` statements is supported for backwards compatibility but
	// will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +optional
	RawAnyAllConditions *apiextv1.JSON `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`
}

func (r *TargetResourceSpec) GetAnyAllConditions() apiextensions.JSON {
	return FromJSON(r.RawAnyAllConditions)
}
