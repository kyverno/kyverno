package v1alpha1

type Policy struct {
	Name      string     `json:"name"`
	Resources []Resource `json:"resources,omitempty"`
	Rules     []Rule     `json:"rules,omitempty"`
}
