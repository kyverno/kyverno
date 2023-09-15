package values

type Resource struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Values map[string]interface{} `json:"values"`
}
