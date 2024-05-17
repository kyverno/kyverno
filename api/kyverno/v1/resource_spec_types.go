package v1

import (
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type ResourceSpec struct {
	// APIVersion specifies resource apiVersion.
	// +optional
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	// Kind specifies resource kind.
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	// Namespace specifies resource namespace.
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Name specifies the resource name.
	// +optional
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// UID specifies the resource uid.
	// +optional
	UID types.UID `json:"uid,omitempty" yaml:"uid,omitempty"`
}

func (s ResourceSpec) GetName() string       { return s.Name }
func (s ResourceSpec) GetNamespace() string  { return s.Namespace }
func (s ResourceSpec) GetKind() string       { return s.Kind }
func (s ResourceSpec) GetAPIVersion() string { return s.APIVersion }
func (s ResourceSpec) GetUID() types.UID     { return s.UID }
func (s ResourceSpec) GetGroupVersion() (schema.GroupVersion, error) {
	return schema.ParseGroupVersion(s.APIVersion)
}

func (s ResourceSpec) String() string {
	return strings.Join([]string{s.APIVersion, s.Kind, s.Namespace, s.Name}, "/")
}

// TargetResourceSpec defines targets for mutating existing resources.
type TargetResourceSpec struct {
	// ResourceSpec contains the target resources to load when mutating existing resources.
	ResourceSpec `json:",omitempty" yaml:",omitempty"`

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
