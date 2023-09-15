package v1alpha1

type ValuesSpec struct {
	Policies []Policy `json:"policies,omitempty"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	GlobalValues       map[string]interface{} `json:"globalValues,omitempty"`
	NamespaceSelectors []NamespaceSelector    `json:"namespaceSelector,omitempty"`
	Subresources       []Subresource          `json:"subresources,omitempty"`
}
