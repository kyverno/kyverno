package v1alpha1

// Rule declares values for a given policy rule
type Rule struct {
	// Name is the name of the ppolicy rule
	Name string `json:"name"`

	// Values are the values for the given policy rule
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Values map[string]interface{} `json:"values,omitempty"`

	// ForeachValues are the foreach values for the given policy rule
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	ForeachValues map[string][]interface{} `json:"foreachValues,omitempty"`
}
