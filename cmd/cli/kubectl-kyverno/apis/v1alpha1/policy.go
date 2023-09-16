package v1alpha1

// Policy declares values for a given policy
type Policy struct {
	// Name is the policy name
	Name string `json:"name"`

	// Resources are values for specific resources
	Resources []Resource `json:"resources,omitempty"`

	// Rules are values for specific policy rules
	Rules []Rule `json:"rules,omitempty"`
}
