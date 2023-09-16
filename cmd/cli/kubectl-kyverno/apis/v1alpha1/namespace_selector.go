package v1alpha1

// NamespaceSelector declares labels for a given namespace
type NamespaceSelector struct {
	// Name is the namespace name
	Name string `json:"name"`

	// Labels are the labels for the given namespace
	Labels map[string]string `json:"labels"`
}
