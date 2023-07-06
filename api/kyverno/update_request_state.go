package kyverno

// UpdateRequestState defines the state of request.
type UpdateRequestState string

const (
	// Pending - the Request is yet to be processed or resource has not been created.
	Pending UpdateRequestState = "Pending"

	// Failed - the Update Request Controller failed to process the rules.
	Failed UpdateRequestState = "Failed"

	// Completed - the Update Request Controller created resources defined in the policy.
	Completed UpdateRequestState = "Completed"

	// Skip - the Update Request Controller skips to generate the resource.
	Skip UpdateRequestState = "Skip"
)
