package v1alpha1

type Rule struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Values map[string]interface{} `json:"values,omitempty"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	ForeachValues map[string][]interface{} `json:"foreachValues,omitempty"`
}
