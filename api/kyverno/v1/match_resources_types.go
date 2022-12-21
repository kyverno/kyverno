package v1

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// MatchResources is used to specify resource and admission review request data for
// which a policy rule is applicable.
type MatchResources struct {
	// Any allows specifying resources which will be ORed
	// +optional
	Any ResourceFilters `json:"any,omitempty" yaml:"any,omitempty"`

	// All allows specifying resources which will be ANDed
	// +optional
	All ResourceFilters `json:"all,omitempty" yaml:"all,omitempty"`

	// UserInfo contains information about the user performing the operation.
	// Specifying UserInfo directly under match is being deprecated.
	// Please specify under "any" or "all" instead.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
	// Requires at least one tag to be specified when under MatchResources.
	// Specifying ResourceDescription directly under match is being deprecated.
	// Please specify under "any" or "all" instead.
	// +optional
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// GetKinds returns all kinds
func (m *MatchResources) GetKinds() []string {
	var kinds []string
	kinds = append(kinds, m.ResourceDescription.Kinds...)
	for _, value := range m.All {
		kinds = append(kinds, value.ResourceDescription.Kinds...)
	}
	for _, value := range m.Any {
		kinds = append(kinds, value.ResourceDescription.Kinds...)
	}
	return kinds
}

// Validate implements programmatic validation
func (m *MatchResources) Validate(path *field.Path, namespaced bool, clusterResources sets.Set[string]) (errs field.ErrorList) {
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
	errs = append(errs, m.UserInfo.Validate(path)...)
	errs = append(errs, m.ResourceDescription.Validate(path, namespaced, clusterResources)...)
	return errs
}
