package v1alpha1

// NamespaceSelector declares values to be loaded by the Kyverno CLI.
type NamespaceSelector struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}
