package v1alpha1

// Resource declares values for a given resource
type Resource struct {
	// Name is the name of the resource
	Name string `json:"name"`

	// Values are the values for the given resource
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Values map[string]interface{} `json:"values,omitempty"`
}
