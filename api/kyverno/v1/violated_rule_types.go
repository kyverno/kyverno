package v1

// ViolatedRule stores the information regarding the rule.
type ViolatedRule struct {
	// Name specifies violated rule name.
	Name string `json:"name" yaml:"name"`

	// Type specifies violated rule type.
	Type string `json:"type" yaml:"type"`

	// Message specifies violation message.
	// +optional
	Message string `json:"message" yaml:"message"`

	// Status shows the rule response status
	Status string `json:"status" yaml:"status"`
}
