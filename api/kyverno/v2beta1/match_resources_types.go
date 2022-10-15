package v2beta1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// MatchResources is used to specify resource and admission review request data for
// which a policy rule is applicable.
type MatchResources struct {
	// RequestTypes can contain values ["CREATE, "UPDATE", "CONNECT", "DELETE"], which are used to match a specific action.
	// +kubebuilder:validation:MaxItems=3
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Optional
	RequestTypes []string `json:"requestTypes,omitempty" yaml:"requestTypes,omitempty"`
	// Any allows specifying resources which will be ORed
	// Any allows specifying resources which will be ORed
	// +optional
	Any kyvernov1.ResourceFilters `json:"any,omitempty" yaml:"any,omitempty"`

	// All allows specifying resources which will be ANDed
	// +optional
	All kyvernov1.ResourceFilters `json:"all,omitempty" yaml:"all,omitempty"`
}

// GetKinds returns all kinds
func (m *MatchResources) GetKinds() []string {
	var kinds []string
	for _, value := range m.All {
		kinds = append(kinds, value.ResourceDescription.Kinds...)
	}
	for _, value := range m.Any {
		kinds = append(kinds, value.ResourceDescription.Kinds...)
	}
	return kinds
}

// Validate implements programmatic validation
func (m *MatchResources) Validate(path *field.Path, namespaced bool, clusterResources sets.String) (errs field.ErrorList) {
	if len(m.Any) > 0 && len(m.All) > 0 {
		errs = append(errs, field.Invalid(path, m, "Can't specify any and all together"))
	}
	anyPath := path.Child("any")
	for i, filter := range m.Any {
		errs = append(errs, filter.UserInfo.Validate(anyPath.Index(i))...)
		errs = append(errs, filter.ResourceDescription.Validate(anyPath.Index(i), namespaced, clusterResources)...)
	}
	allPath := path.Child("all")
	for i, filter := range m.All {
		errs = append(errs, filter.UserInfo.Validate(anyPath.Index(i))...)
		errs = append(errs, filter.ResourceDescription.Validate(allPath.Index(i), namespaced, clusterResources)...)
	}
	return errs
}
