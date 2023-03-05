package v1

import "strings"

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
}

func (s ResourceSpec) GetName() string       { return s.Name }
func (s ResourceSpec) GetNamespace() string  { return s.Namespace }
func (s ResourceSpec) GetKind() string       { return s.Kind }
func (s ResourceSpec) GetAPIVersion() string { return s.APIVersion }

func (s ResourceSpec) String() string {
	return strings.Join([]string{s.APIVersion, s.Kind, s.Namespace, s.Name}, "/")
}
