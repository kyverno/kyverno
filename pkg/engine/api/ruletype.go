package api

// RuleType represents the type of a rule
type RuleType string

const (
	// Mutation type for mutation rule
	Mutation RuleType = "Mutation"
	// Validation type for validation rule
	Validation RuleType = "Validation"
	// Generation type for generation rule
	Generation RuleType = "Generation"
	// ImageVerify type for image verification
	ImageVerify RuleType = "ImageVerify"
)
