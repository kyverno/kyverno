package kyverno

// UpdateRequestStatus defines the observed state of UpdateRequest
type UpdateRequestStatus struct {
	// Handler represents the instance ID that handles the UR
	// Deprecated
	Handler string `json:"handler,omitempty" yaml:"handler,omitempty"`

	// State represents state of the update request.
	State UpdateRequestState `json:"state" yaml:"state"`

	// Specifies request status message.
	// +optional
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// This will track the resources that are updated by the generate Policy.
	// Will be used during clean up resources.
	GeneratedResources []ResourceSpec `json:"generatedResources,omitempty" yaml:"generatedResources,omitempty"`
}
